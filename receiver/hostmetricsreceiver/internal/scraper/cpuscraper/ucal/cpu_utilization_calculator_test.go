// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ucal

import (
	"testing"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
)

type inMemoryRecorder struct {
	cpuUtilizations []CPUUtilization
}

func (r *inMemoryRecorder) record(_ pdata.Timestamp, utilization CPUUtilization) {
	r.cpuUtilizations = append(r.cpuUtilizations, utilization)
}

func TestCpuUtilizationCalculator_Calculate(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name                 string
		now                  pdata.Timestamp
		previousTime         pdata.Timestamp
		cpuTimes             []cpu.TimesStat
		previousCPUTimes     []cpu.TimesStat
		expectedUtilizations []CPUUtilization
		expectedError        error
	}{
		{
			name: "no previous times",
			now:  1640097435776827000,
			cpuTimes: []cpu.TimesStat{
				{
					CPU:  "cpu0",
					User: 8260.4,
				},
			},
		},
		{
			name:         "no delta time",
			previousTime: 1640097430772858000,
			now:          1640097430772858000,
			previousCPUTimes: []cpu.TimesStat{
				{
					CPU:  "cpu0",
					User: 8259.4,
				},
			},
			cpuTimes: []cpu.TimesStat{
				{
					CPU:  "cpu0",
					User: 8260.4,
				},
			},
			expectedError: ErrInvalidElapsed,
		},
		{
			name:         "invalid TimesStats",
			previousTime: 1640097430772858000,
			now:          1640097430772859000,
			previousCPUTimes: []cpu.TimesStat{
				{
					CPU:  "cpu5",
					User: 8259.4,
				},
			},
			cpuTimes: []cpu.TimesStat{
				{
					CPU:  "cpu6",
					User: 8260.4,
				},
			},
			expectedError: ErrTimeStatNotFound,
		},
		{
			name:         "one cpu",
			previousTime: 1640097430772858000,
			now:          1640097435776827000,
			previousCPUTimes: []cpu.TimesStat{
				{
					CPU:    "cpu0",
					User:   8258.4,
					System: 6193.3,
					Idle:   34284.7,
				},
			},
			cpuTimes: []cpu.TimesStat{
				{
					CPU:    "cpu0",
					User:   8259.4,
					System: 6193.9,
					Idle:   34288.2,
				},
			},
			expectedUtilizations: []CPUUtilization{
				{
					CPU:    "cpu0",
					User:   0.19984,
					System: 0.11990,
					Idle:   0.69944,
				},
			},
		},
		{
			name:         "multiple cpus unordered",
			previousTime: 1640097430772858000,
			now:          1640097435776827000,
			previousCPUTimes: []cpu.TimesStat{
				{
					CPU:    "cpu1",
					User:   528.3,
					System: 549.7,
					Idle:   47638.2,
				},
				{
					CPU:    "cpu0",
					User:   8258.4,
					System: 6193.3,
					Idle:   34284.7,
				},
			},
			cpuTimes: []cpu.TimesStat{
				{
					CPU:    "cpu0",
					User:   8259.4,
					System: 6193.9,
					Idle:   34288.2,
				},
				{
					CPU:    "cpu1",
					User:   528.4,
					System: 549.7,
					Idle:   47643.1,
				},
			},
			expectedUtilizations: []CPUUtilization{
				{
					CPU:    "cpu1",
					User:   0.01998,
					System: 0,
					Idle:   0.97922,
				},
				{
					CPU:    "cpu0",
					User:   0.19984,
					System: 0.11990,
					Idle:   0.69944,
				},
			},
		},
	}
	for _, test := range testCases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			recorder := inMemoryRecorder{}
			calculator := CPUUtilizationCalculator{
				previousTime:     test.previousTime,
				previousCPUTimes: test.previousCPUTimes,
			}
			err := calculator.CalculateAndRecord(test.now, test.cpuTimes, recorder.record)
			assert.ErrorIs(t, err, test.expectedError)
			assert.Len(t, recorder.cpuUtilizations, len(test.expectedUtilizations))
			for idx, expectedUtilization := range test.expectedUtilizations {
				assert.Equal(t, expectedUtilization.CPU, recorder.cpuUtilizations[idx].CPU)
				assert.InDelta(t, expectedUtilization.System, recorder.cpuUtilizations[idx].System, 0.00001)
				assert.InDelta(t, expectedUtilization.User, recorder.cpuUtilizations[idx].User, 0.00001)
				assert.InDelta(t, expectedUtilization.Idle, recorder.cpuUtilizations[idx].Idle, 0.00001)
			}
		})
	}
}

func Test_cpuUtilization(t *testing.T) {

	elapsedSeconds := 5.0
	timeStart := cpu.TimesStat{
		CPU:    "cpu0",
		User:   1.5,
		System: 2.7,
		Idle:   0.8,
	}
	timeEnd := cpu.TimesStat{
		CPU:    "cpu0",
		User:   2.7,
		System: 4.2,
		Idle:   3.1,
	}
	expectedUtilization := CPUUtilization{
		CPU:    "cpu0",
		User:   0.24,
		System: 0.3,
		Idle:   0.46,
	}

	actualUtilization := cpuUtilization(timeStart, timeEnd, elapsedSeconds)
	assert.Equal(t, expectedUtilization.CPU, actualUtilization.CPU, 0.00001)
	assert.InDelta(t, expectedUtilization.User, actualUtilization.User, 0.00001)
	assert.InDelta(t, expectedUtilization.System, actualUtilization.System, 0.00001)
	assert.InDelta(t, expectedUtilization.Idle, actualUtilization.Idle, 0.00001)

}

func Test_cpuTimeByCpu(t *testing.T) {
	testCases := []struct {
		name             string
		cpuNum           string
		times            []cpu.TimesStat
		expectedErr      error
		expectedTimeStat cpu.TimesStat
	}{
		{
			name:        "cpu does not exist",
			cpuNum:      "cpu9",
			times:       []cpu.TimesStat{{CPU: "cpu0"}, {CPU: "cpu1"}, {CPU: "cpu2"}},
			expectedErr: ErrTimeStatNotFound,
		},
		{
			name:             "cpu does exist",
			cpuNum:           "cpu1",
			times:            []cpu.TimesStat{{CPU: "cpu0"}, {CPU: "cpu1"}, {CPU: "cpu2"}},
			expectedTimeStat: cpu.TimesStat{CPU: "cpu1"},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			actualTimeStat, err := cpuTimeForCPU(test.cpuNum, test.times)
			assert.ErrorIs(t, err, test.expectedErr)
			assert.Equal(t, test.expectedTimeStat, actualTimeStat)
		})
	}
}
