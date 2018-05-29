// Package errors implements orbit error handling and logging.
package errors

import (
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/laxmicoinofficial/go/clients/orbit"
)

func GetTxErrorResultCodes(err error, logger log.Logger) *orbit.TransactionResultCodes {
	level.Error(logger).Log("msg", err)
	switch e := err.(type) {
	case *orbit.Error:
		code, err := e.ResultCodes()
		if err != nil {
			level.Error(logger).Log("msg", "failed to extract result codes from orbit response")
			return nil
		}
		level.Error(logger).Log("code", code.TransactionCode)
		for i, opCode := range code.OperationCodes {
			level.Error(logger).Log("opcode_index", i, "opcode", opCode)
		}

		return code
	}
	return nil
}
