package worker

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/jszwec/csvutil"
	"github.com/pkg/errors"

	"ftp-worker/pkg/cftp"
	"ftp-worker/pkg/clogger"
	"ftp-worker/pkg/cryptoservice"
	"ftp-worker/pkg/eapi"
	"ftp-worker/pkg/workerpool"

	"ftp-worker/internal/reestr"
	"ftp-worker/internal/store/service"
)

type Worker struct {
	StoreDB    *service.Store
	Eapi       *eapi.Client
	Crypto     *cryptoservice.Client
	FTPCfg     *cftp.Config
	FTPPoolCfg *cftp.PoolConfig
	Log        *clogger.Logger
	connPool   *cftp.Pool
}

func (w *Worker) Shutdown(ctx context.Context) {
	if w.connPool != nil {
		err := w.connPool.Close()

		w.Log.Error().Err(err).Msg("Failed to close FTP connection pool")
	}

	w.Log.Info().Msg("Worker shutdown complete")
}

func (w *Worker) Do(gctx context.Context) {
	w.Log.Info().Msg("Starting worker")

	ctx, cancel := context.WithCancel(gctx)
	defer cancel()

	w.connPool = cftp.NewPool(ctx,
		cancel,
		func(ctx context.Context) (*ftp.ServerConn, int, error) {
			conn, stat, err := cftp.CreateConn(ctx, w.FTPCfg)
			if err != nil {
				return nil, stat, errors.Wrap(err, "create ftp connection")
			}

			return conn, stat, nil
		},
		w.FTPPoolCfg)

	conn, stat, err := w.connPool.Get(ctx)
	if err != nil {
		w.Log.Error().Err(err).Msgf("Failed to get FTP connection from pool with status code %d", stat)

		return
	}

	rootEntries, err := conn.List(w.FTPCfg.Root)
	if err != nil {
		w.Log.Error().Err(err).Msg("Failed to list root directory")

		return
	}

	for _, rootEntry := range rootEntries {
		if rootEntry.Type != ftp.EntryTypeFolder {
			continue
		}

		ts, err := parseTime(rootEntry.Name)
		if err != nil {
			w.Log.Error().Err(err).Str("folder", rootEntry.Name).Msg("Failed to parse folder name")

			continue
		}

		entries, err := conn.List(w.FTPCfg.Root + "/" + rootEntry.Name)
		if err != nil {
			w.Log.Error().Err(err).Str("folder", rootEntry.Name).Msg("Failed to list folder")

			continue
		}

		for _, entry := range entries {
			if entry.Type != ftp.EntryTypeFile || !strings.HasSuffix(entry.Name, ".csv") {
				continue
			}

			doneList, err := w.StoreDB.ListDoneByDir(ctx, rootEntry.Name)
			if err != nil && !errors.Is(err, w.StoreDB.ErrNotFound()) {
				w.Log.Error().Err(err).Str("folder", rootEntry.Name).Msg("Failed to list done files")

				continue
			}

			w.processReestr(ctx, conn, rootEntry.Name, entry.Name, doneList, ts, "add")
			w.processReestr(ctx, conn, rootEntry.Name, entry.Name, doneList, ts, "delete")
		}
	}

	w.connPool.Put(conn)

	w.Log.Info().Msg("Worker done")
}

func (w *Worker) processReestr(ctx context.Context, conn *ftp.ServerConn, rootEntryName, entryName string, doneList []*reestr.WorkResult, ts time.Time, op string) {
	if strings.HasPrefix(entryName, op) {
		reestrAdd, err := w.getReestr(conn, w.FTPCfg.Root+rootEntryName+"/"+entryName)
		if err != nil {
			w.Log.Error().Err(err).Str("file", entryName).Msg("Failed to get reestr")

			return
		}

		preparedList := removeElements(reestrAdd, doneList, rootEntryName, op)

		if len(preparedList) == 0 {
			return
		}

		switch op {
		case "add":
			w.Log.Info().Int("count", len(preparedList)).Str("file", entryName).Msg("Adding accounts")

			w.workerReestr(ctx, preparedList, ts, rootEntryName, op, func(val *reestr.WorkResult) *reestr.WorkResult {
				return w.addJobFn(ctx, val, ts, rootEntryName)
			})

		case "delete":
			w.Log.Info().Int("count", len(preparedList)).Str("file", entryName).Msg("Deleting accounts")

			w.workerReestr(ctx, preparedList, ts, rootEntryName, op, func(val *reestr.WorkResult) *reestr.WorkResult {
				return w.delJobFn(ctx, val)
			})
		}
	}
}

func (w *Worker) workerReestr(ctx context.Context, inputData []*reestr.WorkResult, ts time.Time, dir, op string, jobFn func(*reestr.WorkResult) *reestr.WorkResult) {
	size := w.FTPPoolCfg.MaxTotal - 1

	wp := workerpool.New[*reestr.WorkResult, *reestr.WorkResult](size*2, size)

	var inputCounter int

	inputFn := func() (*reestr.WorkResult, bool) {
		if inputCounter >= len(inputData) {
			return &reestr.WorkResult{}, false
		}

		data := inputData[inputCounter]

		inputCounter++

		return data, true
	}

	outputFn := func(val *reestr.WorkResult) {
		w.outputFn(ctx, val, op)
	}

	wp.Do(ctx, inputFn, jobFn, outputFn, func(err error) {
		w.Log.Error().Err(err).Msg("Error in worker pool")
	})
}

func (w *Worker) addJobFn(ctx context.Context, val *reestr.WorkResult, ts time.Time, path string) *reestr.WorkResult {
	conn, stat, err := w.connPool.Get(ctx)
	if err != nil {
		w.Log.Error().Err(err).Msgf("Failed to create FTP connection with status code %d", stat)

		val.ErrorMesage = err.Error()
		val.StatusCode = stat

		return val
	}

	size, err := conn.FileSize(w.FTPCfg.Root + path + "/" + val.FileName)
	if err != nil {
		w.Log.Error().Err(err).Msg("Failed to get file size")

		val.ErrorMesage = err.Error()
		val.StatusCode = http.StatusBadRequest

		return val
	}

	if size >= int64(w.Crypto.Cfg.MaxMessageSize) {
		err := errors.New("photo " + val.FileName + " oversize")

		w.Log.Error().Err(err).Msg("Photo oversize")

		val.ErrorMesage = err.Error()
		val.StatusCode = http.StatusBadRequest

		return val
	}

	req, err := conn.Retr(w.FTPCfg.Root + path + "/" + val.FileName)
	if err != nil {
		w.Log.Error().Err(err).Msg("Failed to retrieve file from FTP")

		val.ErrorMesage = err.Error()
		val.StatusCode = http.StatusInternalServerError

		w.Log.Info().Msgf("Worker %+v\n", val)

		return val
	}

	photo, err := io.ReadAll(req)
	if err != nil {
		w.Log.Error().Err(err).Msg("Failed to read photo data")

		val.ErrorMesage = err.Error()
		val.StatusCode = http.StatusInternalServerError

		return val
	}

	err = req.Close()
	if err != nil {
		w.Log.Error().Err(err).Msg("Failed to close FTP file reader")
	}

	w.connPool.Put(conn)

	signature, err := w.Crypto.SignDetachedDataWithCert(ctx, photo, "GOST12_256", "PLAIN_PKCS7", w.Crypto.Cfg.Certificate)
	if err != nil {
		w.Log.Error().Err(err).Msg("Failed to sign photo data")

		val.ErrorMesage = err.Error()
		val.StatusCode = http.StatusInternalServerError

		return val
	}

	params := eapi.NewFields()

	params.Sub = val.FpID
	params.Iat = time.Now().UnixMilli()
	params.Exp = params.Iat + time.Duration(time.Minute*time.Duration(3)).Milliseconds()

	if val.AgreeDateFrom != 0 {
		params.Agree = &eapi.Agree{
			AgreementID: val.AgreeID,
			DateFrom:    time.Unix(int64(val.AgreeDateFrom), 0),
		}
	} else {
		val.StatusCode = http.StatusBadRequest
		val.ErrorMesage = "agreement date_from must not be null"

		return val
	}

	params.ExternalAuthPersonalFields = &eapi.ExternalAuthPersonalFields{
		ExternalAuthPerson: []eapi.ExternalAuthPerson{
			{Key: w.Eapi.Cfg.Provider, Value: val.SudirID},
		},
	}

	params.DatetimeTz = ts.Unix()

	params.BiometricCollecting = []eapi.BiometricCollecting{
		{
			Name:      val.FileName,
			Modality:  "photo",
			Signature: string(signature),
		},
	}

	statusCode, err := w.Eapi.BiometricSample(ctx, val.FileName, val.FileName, photo, params)
	val.StatusCode = statusCode

	if err != nil {
		w.Log.Error().Err(err).Msg("Failed to send biometric sample")

		val.ErrorMesage = err.Error()
	}

	return val
}

func (w *Worker) delJobFn(ctx context.Context, val *reestr.WorkResult) *reestr.WorkResult {
	iat := time.Now().UnixMilli()

	params := eapi.SubFields{
		Sub: val.FpID,
		Iat: iat,
		Exp: iat + time.Duration(time.Minute*time.Duration(3)).Milliseconds(),
	}

	paramsStr, err := json.Marshal(params)
	if err != nil {
		w.Log.Error().Err(err).Msg("Failed to marshal JWT payload")

		val.ErrorMesage = err.Error()
		val.StatusCode = http.StatusInternalServerError

		return val
	}

	jwt, err := w.Crypto.CreateJWTWithCert(ctx, string(paramsStr), "GOST12_256", "PLAIN_PKCS7", w.Crypto.Cfg.Certificate)
	if err != nil {
		w.Log.Error().Err(err).Msg("Failed to create JWT")

		val.ErrorMesage = err.Error()
		val.StatusCode = http.StatusInternalServerError

		w.Log.Info().Msgf("Worker %+v\n", val)

		return val
	}

	statusCode, err := w.Eapi.Del(ctx, jwt)
	val.StatusCode = statusCode

	if err != nil {
		w.Log.Error().Err(err).Msg("Failed to delete biometric sample")

		val.ErrorMesage = err.Error()
	}

	return val
}

func (w *Worker) outputFn(ctx context.Context, val *reestr.WorkResult, op string) {
	if val == nil {
		return
	}

	r, err := w.StoreDB.ReadByDirOpFpID(ctx, val.FpID, val.Dir, op)
	if err != nil && !errors.Is(err, w.StoreDB.ErrNotFound()) {
		w.Log.Error().Err(err).Msg("Failed to read work result")
	}

	if errors.Is(err, w.StoreDB.ErrNotFound()) {
		val, err = w.StoreDB.Create(ctx, val)
		if err != nil {
			w.Log.Error().Err(err).Msg("Failed to create work result")
		}
	} else {
		val.ID = r.ID

		err = w.StoreDB.Update(ctx, val)
		if err != nil {
			w.Log.Error().Err(err).Msg("Failed to update work result")
		}
	}

	w.Log.Info().Msgf("Worker %+v\n", val)
}

func (w *Worker) getReestr(conn *ftp.ServerConn, fileName string) ([]reestr.AddRow, error) {
	var records []reestr.AddRow

	req, err := conn.Retr(fileName)
	if err != nil {
		if strings.HasPrefix(err.Error(), "550 Failed to open file") {
			return records, nil
		}

		return nil, errors.Wrap(err, "ftp download reestr")
	}

	buf, err := io.ReadAll(req)
	if err != nil {
		return nil, errors.Wrap(err, "ftp download reestr")
	}

	err = req.Close()
	if err != nil {
		w.Log.Error().Err(err).Str("file", fileName).Msg("Failed to close FTP file reader")
	}

	if err := csvutil.Unmarshal(buf, &records); err != nil {
		return nil, errors.Wrap(err, "ftp download reestr")
	}

	return records, nil
}

func parseTime(name string) (time.Time, error) {
	var err error

	ps := strings.Split(name, "_")

	if len(ps) != 6 {
		return time.Time{}, errors.New("bad folder name")
	}

	p := make([]int, 6)

	for i, v := range ps {
		if i < 6 {
			p[i], err = strconv.Atoi(v)
			if err != nil {
				return time.Time{}, errors.Wrap(err, "parse time")
			}
		}
	}

	return time.Date(p[0], time.Month(p[1]), p[2], p[3], p[4], p[5], 0, time.Local), nil
}

func removeElements(slice []reestr.AddRow, toRemove []*reestr.WorkResult, dir, op string) []*reestr.WorkResult {
	result := []*reestr.WorkResult{}

	removeMap := make(map[string]bool)

	for _, val := range toRemove {
		removeMap[val.FpID] = true
	}

	for _, val := range slice {
		if !removeMap[val.FpID] {
			vv := &reestr.WorkResult{
				Dir:       dir,
				Operation: op,
			}

			vv.AddRow = val

			result = append(result, vv)
		}
	}

	return result
}
