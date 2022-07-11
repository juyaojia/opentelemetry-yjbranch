// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package correlation

import (
	"errors"
	"testing"

	"github.com/signalfx/signalfx-agent/pkg/apm/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func newShimTest() (*observer.ObservedLogs, zapShim) {
	logger, observed := observer.New(zap.DebugLevel)
	z := zap.New(logger)
	zs := newZapShim(z)
	return observed, zs
}

func TestZapShim_Error(t *testing.T) {
	observed, zs := newShimTest()

	zs.WithError(errors.New("logged error")).Error("error message")
	logs := observed.TakeAll()
	require.Len(t, logs, 1)
	e := logs[0]
	assert.Equal(t, "error message", e.Message)
	assert.Equal(t, zap.ErrorLevel, e.Level)
	assert.Len(t, e.Context, 1)
	c := e.Context[0]
	assert.Equal(t, "error", c.Key)
	require.Equal(t, zapcore.ErrorType, c.Type)
	assert.EqualError(t, c.Interface.(error), "logged error")
}

func TestZapShim_Debug(t *testing.T) {
	observed, zs := newShimTest()

	zs.Debug("debug message")
	logs := observed.TakeAll()
	require.Len(t, logs, 1)
	e := logs[0]
	assert.Equal(t, "debug message", e.Message)
	assert.Equal(t, zap.DebugLevel, e.Level)
	assert.Len(t, e.Context, 0)
}

func TestZapShim_Warn(t *testing.T) {
	observed, zs := newShimTest()

	zs.Warn("warn message")
	logs := observed.TakeAll()
	require.Len(t, logs, 1)
	e := logs[0]
	assert.Equal(t, "warn message", e.Message)
	assert.Equal(t, zap.WarnLevel, e.Level)
	assert.Len(t, e.Context, 0)
}

func TestZapShim_Info(t *testing.T) {
	observed, zs := newShimTest()

	zs.Info("info message")
	logs := observed.TakeAll()
	require.Len(t, logs, 1)
	e := logs[0]
	assert.Equal(t, "info message", e.Message)
	assert.Equal(t, zap.InfoLevel, e.Level)
	assert.Len(t, e.Context, 0)
}

func TestZapShim_Panic(t *testing.T) {
	observed, zs := newShimTest()

	assert.Panics(t, func() {
		zs.Panic("panic message")
	})
	logs := observed.TakeAll()
	require.Len(t, logs, 1)
	e := logs[0]
	assert.Equal(t, "panic message", e.Message)
	assert.Equal(t, zap.PanicLevel, e.Level)
	assert.Len(t, e.Context, 0)
}

func TestZapShim_Fields(t *testing.T) {
	observed, zs := newShimTest()

	zs.WithFields(log.Fields{"field": "field value"}).Info("info message with fields")
	logs := observed.TakeAll()
	require.Len(t, logs, 1)
	e := logs[0]
	assert.Equal(t, "info message with fields", e.Message)
	assert.Equal(t, zap.InfoLevel, e.Level)
	assert.Len(t, e.Context, 1)
	c := e.Context[0]
	assert.Equal(t, "field", c.Key)
	require.Equal(t, zapcore.StringType, c.Type)
	assert.Equal(t, c.String, "field value")
}
