// Copyright The OpenTelemetry Authors
// Copyright (c) 2018 The Jaeger Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tracegen // import "github.com/open-telemetry/opentelemetry-collector-contrib/tracegen/internal/tracegen"

import (
	"context"
	"encoding/json"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type Span struct {
	serviceName string
	// attributes  []string
	spanId   string // hash
	parentId string // like a hash
}

type worker struct {
	running          *uint32         // pointer to shared flag that indicates it's time to stop the test
	numTraces        int             // how many traces the worker has to generate (only when duration==0)
	propagateContext bool            // whether the worker needs to propagate the trace context via HTTP headers
	totalDuration    time.Duration   // how long to run the test for (overrides `numTraces`)
	limitPerSecond   rate.Limit      // how many spans per second to generate
	wg               *sync.WaitGroup // notify when done
	logger           *zap.Logger
	traceTypes       int
	serviceNames     [12]string
	tracerProviders  []*sdktrace.TracerProvider
}

const (
	fakeIP string = "1.2.3.4"

	fakeSpanDuration = 100000 * time.Microsecond
)

func (w worker) setUpTracers() []trace.Tracer {
	toReturn := make([]trace.Tracer, 0, len(w.tracerProviders))

	for i := 0; i < len(w.tracerProviders); i++ {
		otel.SetTracerProvider(w.tracerProviders[i])
		tracer := otel.Tracer("tracegen" + string(i))
		toReturn = append(toReturn, tracer)
	}
	return toReturn
}

func (w worker) addChild(parentCtx context.Context, tracer trace.Tracer, message string, serviceName string, httpStatusCode string, httpUrl string) context.Context {
	childCtx, child := tracer.Start(parentCtx, message, trace.WithAttributes(
		attribute.String("span.kind", getRandSpanKind()), // is there a semantic convention for this?
		attribute.String("service.name", serviceName),
		semconv.HTTPStatusCodeKey.String(httpStatusCode),
		semconv.HTTPURLKey.String(httpUrl),
	))
	opt := trace.WithTimestamp(time.Now().Add(fakeSpanDuration))
	child.End(opt)
	return childCtx
}

// input a range and get a random number within that range
func getRandomNum(min int, max int) int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(max-min+1) + min
}

// get a status code based on hardcoded probability
func getRandStatusCode() string {
	var statusCode string
	randNum := getRandomNum(1, 10)
	switch randNum {
	case 1, 2, 3, 4, 5:
		statusCode = "200"
	case 6:
		statusCode = "500"
	case 7:
		statusCode = "202"
	case 8:
		statusCode = "402"
	case 9:
		statusCode = "300"
	case 10:
		statusCode = "404"
	// just in case of error
	default:
		statusCode = "Error"
	}
	return statusCode
}

// get stub span kind
func getRandSpanKind() string {
	var spanKind string
	randNum := getRandomNum(1, 2)
	switch randNum {
	case 1:
		spanKind = "client"
	case 2:
		spanKind = "server"
	// just in case of error
	default:
		spanKind = "N/A"
	}
	return spanKind
}

// need exception handling!
func (w worker) getRootAttribute(servicesIndex int) (string, string, string, string) {
	//one status code and url for entire tree
	spanKind := getRandSpanKind()
	serviceName := w.serviceNames[servicesIndex]
	httpStatusCode := getRandStatusCode()
	httpUrl := "http://metadata.google.internal/computeMetadata/v1/instance/attributes/cluster-name(fakeurl)"
	return spanKind, serviceName, httpStatusCode, httpUrl
}

// perform preparation and start the trace generation
func (w worker) simulateTraces() {
	// set up all tracers
	tracers := w.setUpTracers()
	limiter := rate.NewLimiter(w.limitPerSecond, 1)

	// entry point!
	filename := "test1.json"
	spans, childrenList := extractSpans(filename)

	var i int
	for atomic.LoadUint32(w.running) == 1 {
		w.generateTraceHelper(spans, limiter, tracers, childrenList)
		i++
		if w.numTraces != 0 {
			if i >= w.numTraces {
				break
			}
		}
	}
	w.logger.Info("traces generated", zap.Int("traces", i))
	w.wg.Done()
}

// find the root span of all the spans in the trace
func findRoot(spans []Span) int {
	for i := 0; i < len(spans); i++ {
		if len(spans[i].parentId) == 0 {
			return i
		}
	}
	return -1
}

// find the index of the serviceName in serviceName, will be used to index tracer as well
func findIndex(target string, serviceName [12]string) int {
	for i := 0; i < len(serviceName); i++ {
		if serviceName[i] == target {
			return i
		}
	}
	return -1
}

// generate the Trace
func (w worker) generateTrace(parentCtx context.Context, spanIndex int, limiter *rate.Limiter, tracers []trace.Tracer, httpStatusCode string, httpUrl string, spans []Span, childrenList [][]int) {
	// base case
	if len(childrenList[spanIndex]) <= 0 {
		return
	}

	for i := 0; i < len(childrenList[spanIndex]); i++ {
		tracerIndex := findIndex(spans[childrenList[spanIndex][i]].serviceName, w.serviceNames)
		childCtx := w.addChild(parentCtx, tracers[tracerIndex], "message from span "+strconv.Itoa(childrenList[spanIndex][i]), w.serviceNames[tracerIndex], httpStatusCode, httpUrl)
		w.generateTrace(childCtx, childrenList[spanIndex][i], limiter, tracers, httpStatusCode, httpUrl, spans, childrenList)
	}
}

// create the context for the root and then start generate trace from the root
func (w worker) generateTraceHelper(spans []Span, limiter *rate.Limiter, tracers []trace.Tracer, childrenList [][]int) {
	rootIndex := findRoot(spans)
	serviceIndex := findIndex(spans[rootIndex].serviceName, w.serviceNames)
	spanKind, serviceName, httpStatusCode, httpUrl := w.getRootAttribute(serviceIndex)
	ctx, sp := tracers[serviceIndex].Start(context.Background(), "lets-go", trace.WithAttributes(
		attribute.String("span.kind", spanKind), // is there a semantic convention for this?
		attribute.String("service.name", serviceName),
		semconv.HTTPStatusCodeKey.String(httpStatusCode),
		semconv.HTTPURLKey.String(httpUrl),
	))

	w.generateTrace(ctx, rootIndex, limiter, tracers, httpStatusCode, httpUrl, spans, childrenList)
	limiter.Wait(context.Background())
	opt := trace.WithTimestamp(time.Now().Add(fakeSpanDuration))
	sp.End(opt)
}

// Scott's code
// for generalized load generator
// @return: service list - list of services, each being a Service struct containing its spanID, parentID, and processType.
// @return: service child list - lists of children services of the service at the corresponding index in serviceList.
func extractSpans(fileName string) ([]Span, [][]int) {
	content, err := os.ReadFile(fileName)
	if err != nil {
		panic(err)
	}

	serviceList := make([]Span, 0)

	var all map[string]interface{}
	err = json.Unmarshal([]byte(content), &all)
	if err != nil {
		panic(err)
	}

	dataList, ok := all["data"].([]interface{})
	if !ok {
		panic("dataList is not a list!")
	}

	for i := 0; i < len(dataList); i++ {
		dataMap, ok1 := dataList[i].(map[string]interface{})
		if !ok1 {
			panic("dataMap is not a map!")
		}
		spanList, ok2 := dataMap["spans"].([]interface{})
		if !ok2 {
			panic("spanList is not a list!")
		}
		processMap, ok3 := dataMap["processes"].(map[string]interface{})
		if !ok3 {
			panic("processMap is not a map!")
		}

		for j := 0; j < len(spanList); j++ {
			spanMap, ok1 := spanList[j].(map[string]interface{})
			if !ok1 {
				panic("spanMap is not a map!")
			}
			spanID, ok2 := spanMap["spanID"]
			if !ok2 {
				panic("spanMap[spanID] is not a string!")
			}
			var parentID string
			referenceList, ok3 := spanMap["references"].([]interface{})
			if !ok3 {
				panic("referenceList is not a list!")
			}
			if len(referenceList) > 0 {
				referenceMap, ok4 := referenceList[0].(map[string]interface{})
				if !ok4 {
					panic("referenceMap is not a map!")
				}
				parentID = referenceMap["spanID"].(string)
			}

			processTypeMap, ok7 := processMap[spanMap["processID"].(string)].(map[string]interface{})
			if !ok7 {
				panic("processTypeMap is not a map!")
			}
			processType := processTypeMap["serviceName"].(string)

			service := Span{
				spanId:      spanID.(string),
				parentId:    parentID,
				serviceName: processType,
			}
			serviceList = append(serviceList, service)
		}
	}

	serviceChildList := make([][]int, len(serviceList))

	for i := range serviceList {
		parent := serviceList[i].parentId
		if parent != "" {
			for j := range serviceList {
				if serviceList[j].spanId == parent {
					serviceChildList[j] = append(serviceChildList[j], i)
					break
				}
			}
		}
	}

	return serviceList, serviceChildList
}

// Scott's code
