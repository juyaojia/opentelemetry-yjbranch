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
	"math/rand"
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
	attributes  []string
	spanId      string // hash
	parentId    string // like a hash
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

func (w worker) simulateTraces() {
	// set up all tracers
	tracers := w.setUpTracers()
	limiter := rate.NewLimiter(w.limitPerSecond, 1)

	spans := getFakeSpanList()
	childrenList := [][]int{
		{1, 2},
		{3},
		{5, 6},
		{4},
		{},
		{},
	}

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

func findRoot(spans []Span) int {
	for i := 0; i < len(spans); i++ {
		if len(spans[i].parentId) == 0 {
			return i
		}
	}
	return -1
}

func findIndex(target string, serviceName [12]string) int {
	for i := 0; i < len(serviceName); i++ {
		if serviceName[i] == target {
			return i
		}
	}
	return -1
}

func (w worker) generateTrace(parentCtx context.Context, index int, limiter *rate.Limiter, tracers []trace.Tracer, httpStatusCode string, httpUrl string, spans []Span, childrenList [][]int) {
	// base case
	if len(childrenList[index]) <= 0 {
		return
	}
	for i := 0; i < len(childrenList[index]); i++ {
		tracerIndex := findIndex(spans[index].serviceName, w.serviceNames)
		childCtx := w.addChild(parentCtx, tracers[tracerIndex], "message from span "+strconv.Itoa(index), w.serviceNames[tracerIndex], httpStatusCode, httpUrl, spans, childrenList)
		generateTrace(childCtx, childrenList[index][i], limiter, tracers, httpStatusCode, httpUrl, spans, childrenList)
	}
}

func (w worker) generateTraceHelper(spans []Span, limiter *rate.Limiter, tracers []trace.Tracer, childrenList [][]int) {
	rootIndex := findRoot(spans)
	spanKind, serviceName, httpStatusCode, httpUrl := w.getRootAttribute(rootIndex)
	ctx, sp := tracers[rootIndex].Start(context.Background(), "lets-go", trace.WithAttributes(
		attribute.String("span.kind", spanKind), // is there a semantic convention for this?
		attribute.String("service.name", serviceName),
		semconv.HTTPStatusCodeKey.String(httpStatusCode),
		semconv.HTTPURLKey.String(httpUrl),
	))

	generateTrace(ctx, rootIndex, limiter, tracers, httpStatusCode, httpUrl, spans, childrenList)
	limiter.Wait(context.Background())
	opt := trace.WithTimestamp(time.Now().Add(fakeSpanDuration))
	sp.End(opt)
}

func getFakeSpanList() []Span {
	span0 := Span{
		serviceName: "span0",
		attributes:  []string{"a1", "a2", "a3"},
		spanId:      "span0spanidhash",
		parentId:    "",
	}

	span1 := Span{
		serviceName: "span1",
		attributes:  []string{"a1", "a2", "a3"},
		spanId:      "span1spanidhash",
		parentId:    "span0spanidhash",
	}
	span2 := Span{
		serviceName: "span2",
		attributes:  []string{"a1", "a2", "a3"},
		spanId:      "span2spanidhash",
		parentId:    "span0spanidhash",
	}
	span3 := Span{
		serviceName: "span3",
		attributes:  []string{"a1", "a2", "a3"},
		spanId:      "span3spanidhash",
		parentId:    "span1spanidhash",
	}
	span4 := Span{
		serviceName: "span4",
		attributes:  []string{"a1", "a2", "a3"},
		spanId:      "span4spanidhash",
		parentId:    "span3spanidhash",
	}
	span5 := Span{
		serviceName: "span5",
		attributes:  []string{"a1", "a2", "a3"},
		spanId:      "span5spanidhash",
		parentId:    "span2spanidhash",
	}
	span6 := Span{
		serviceName: "span6",
		attributes:  []string{"a1", "a2", "a3"},
		spanId:      "span6spanidhash",
		parentId:    "span2spanidhash",
	}
	spans := []Span{span0, span1, span2, span3, span4, span5, span6}
	return spans
}
