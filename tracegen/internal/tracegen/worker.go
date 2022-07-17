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
        "sync"
        "sync/atomic"
        "time"
        "math/rand"
        "go.opentelemetry.io/otel"
        "go.opentelemetry.io/otel/attribute"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
        semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
        "go.opentelemetry.io/otel/trace"
        "go.uber.org/zap"
        "golang.org/x/time/rate"
)


type worker struct {
        running          *uint32         // pointer to shared flag that indicates it's time to stop the test
        numTraces        int             // how many traces the worker has to generate (only when duration==0)
        propagateContext bool            // whether the worker needs to propagate the trace context via HTTP headers
        totalDuration    time.Duration   // how long to run the test for (overrides `numTraces`)
        limitPerSecond   rate.Limit      // how many spans per second to generate
        wg               *sync.WaitGroup // notify when done
        logger           *zap.Logger
        traceTypes        int
        serviceNames      [12]string
        maxDuration       int
        minDuration       int
    tracerProviders  []*sdktrace.TracerProvider
}

const (
        fakeIP string = "1.2.3.4"

        startTime := time.Now()

        // fakeSpanDuration = 123 * time.Microsecond
)

func (w worker) getSingleFakeSpanDuration(totalSpansDuration int, numSpansinTrace int) int{
    return getRandomNum(w.min, w.max)/numSpansinTrace
}

func (w worker) setUpTracers() []trace.Tracer {
    toReturn := make([]trace.Tracer, 0, len(w.tracerProviders))

    for i := 0; i< len(w.tracerProviders); i++ {
        otel.SetTracerProvider(w.tracerProviders[i])
        tracer := otel.Tracer("tracegen"+string(i))
        toReturn = append(toReturn, tracer)
    }
    return toReturn
}

func (w worker) addChild(parentCtx context.Context, tracer trace.Tracer, message string, serviceName string, httpStatusCode string, httpUrl string, spanStartTime int, spanDuration int) context.Context {
    childCtx, child := tracer.Start(parentCtx, message, trace.WithAttributes(
        attribute.String("span.kind", getRandSpanKind()), // is there a semantic convention for this?
        attribute.String("service.name", serviceName),
        semconv.HTTPStatusCodeKey.String(httpStatusCode),
        semconv.HTTPURLKey.String(httpUrl),
    ), trace.WithTimestamp(spanStartTime))

    opt := trace.WithTimestamp(spanStartTime.Add(spanDuration))
    child.End(opt)
    return childCtx
}

// input a range and get a random number within that range
func getRandomNum(min int, max int) int{
    rand.Seed(time.Now().UnixNano())
    return rand.Intn(max - min+1) + min
}


// get a status code based on hardcoded probability
func getRandStatusCode() string {
    var statusCode string
    randNum := getRandomNum(1, 10)
    switch randNum {
        case 1, 2, 3, 4, 5: statusCode = "200"
        case 6: statusCode = "500"
        case 7: statusCode = "202"
        case 8: statusCode = "402"
        case 9: statusCode = "300"
        case 10: statusCode = "404"
        // just in case of error
        default: statusCode = "Error"
    }
    return statusCode
}

func getRandSpanKind() string {
    var spanKind string
    randNum := getRandomNum(1, 2)
    switch randNum {
        case 1: spanKind = "client"
        case 2: spanKind = "server"
        // just in case of error
        default: spanKind = "N/A"
    }
    return spanKind
}

// need exception handling!
func (w worker) getRootAttribute(servicesIndex int) (string, string, string, string){
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
    var i int
    for atomic.LoadUint32(w.running) == 1 {
        t := i%w.traceTypes
        if t == 0 {
            w.simulateTrace1(limiter, tracers)
        } else if t == 1 {
            w.simulateTrace2(limiter, tracers)
        }
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

func getLayerEndTime(startTime time.Time, singleSpanDuration int, layer int) time.Time{
    if layer <= 0 {
        // exception handling
        return time.Now()
    } else {
        return startTime+singleSpanDuration*layer*time.Microsecond
    }
}
func (w worker) simulateTrace1(limiter *rate.Limiter, tracers []trace.Tracer) {
    spanKind, serviceName, httpStatusCode, httpUrl := w.getRootAttribute(0)
    ctx, sp := tracers[0].Start(context.Background(), "lets-go", trace.WithAttributes(
        attribute.String("span.kind", spanKind), // is there a semantic convention for this?
        attribute.String("service.name", serviceName),
        semconv.HTTPStatusCodeKey.String(httpStatusCode),
        semconv.HTTPURLKey.String(httpUrl),
    ))


    // need to do error checking!
    totalSpans := 24

    traceDuration := getRandomNum(w.minDuration, w.maxDuration)
    singleSpanDuration := getSingleFakeSpanDuration(traceDuration, totalSpans)
    firstLayerDuration := getLayerEndTime(startTime, singleSpanDuration, 1)
    secondLayerDuration := getLayerEndTime(startTime, singleSpanDuration, 2)
    thirdLayerDuration := getLayerEndTime(startTime, singleSpanDuration, 3)
    fourthLayerDuration := getLayerEndTime(startTime, singleSpanDuration, 4)


    child1Ctx := w.addChild(ctx, tracers[0], "1", w.serviceNames[0], httpStatusCode, httpUrl, firstLayerDuration, singleSpanDuration)
    w.addChild(child1Ctx, tracers[4], "11", w.serviceNames[4], httpStatusCode, httpUrl, secondLayerDuration, singleSpanDuration)

    w.addChild(ctx, tracers[0], "2", w.serviceNames[0], httpStatusCode, httpUrl, firstLayerDuration, singleSpanDuration)

    child3Ctx := w.addChild(ctx, tracers[0], "3", w.serviceNames[3], httpStatusCode, httpUrl, firstLayerDuration, singleSpanDuration)
    grandchild3Ctx := w.addChild(child3Ctx, tracers[8], "31", w.serviceNames[8], httpStatusCode, httpUrl, secondLayerDuration, singleSpanDuration)
    greatgrandchild3Ctx := w.addChild(grandchild3Ctx, tracers[8], "311", w.serviceNames[8], httpStatusCode, httpUrl, thirdLayerDuration, singleSpanDuration)
    w.addChild(greatgrandchild3Ctx, tracers[7], "312", w.serviceNames[7], httpStatusCode, httpUrl, fourthLayerDuration, singleSpanDuration)

    for i := 0; i< 5; i++ {
        child4Ctx := w.addChild(ctx, tracers[0], "4", w.serviceNames[0], httpStatusCode, httpUrl, firstLayerDuration, singleSpanDuration)
        w.addChild(child4Ctx, tracers[7], "41", w.serviceNames[7], httpStatusCode, httpUrl, secondLayerDuration, singleSpanDuration)
    }

    child5Ctx := w.addChild(ctx, tracers[0], "5", w.serviceNames[0], httpStatusCode, httpUrl, firstLayerDuration, singleSpanDuration)
    w.addChild(child5Ctx, tracers[10], "51", w.serviceNames[10], httpStatusCode, httpUrl, secondLayerDuration, singleSpanDuration)

    for i := 0; i< 2; i++ {
        child6Ctx := w.addChild(ctx, tracers[0], "6", w.serviceNames[0], httpStatusCode, httpUrl, firstLayerDuration, singleSpanDuration)
        w.addChild(child6Ctx, tracers[4], "61", w.serviceNames[4], httpStatusCode, httpUrl, secondLayerDuration, singleSpanDuration)
    }

    limiter.Wait(context.Background())
    opt := trace.WithTimestamp(startTime.Add(fakeSpanDuration))
    sp.End(opt)
}

func (w worker) simulateTrace2(limiter *rate.Limiter, tracers []trace.Tracer) {
   spanKind, serviceName, httpStatusCode, httpUrl := w.getRootAttribute(0)
   ctx, sp := tracers[0].Start(context.Background(), "lets-go", trace.WithAttributes(
        attribute.String("span.kind", spanKind), // is there a semantic convention for this?
        attribute.String("service.name", serviceName),
        semconv.HTTPStatusCodeKey.String(httpStatusCode),
        semconv.HTTPURLKey.String(httpUrl),
    ), trace.WithTimestamp(startTime))

    // need to do error checking!
    totalSpans := 26

    traceDuration := getRandomNum(w.minDuration, w.maxDuration)
    singleSpanDuration := getSingleFakeSpanDuration(traceDuration, totalSpans)
    firstLayerDuration := getLayerEndTime(startTime, singleSpanDuration, 1)
    secondLayerDuration := getLayerEndTime(startTime, singleSpanDuration, 2)


    child1Ctx := w.addChild(ctx, tracers[0], "1", w.serviceNames[0],httpStatusCode, httpUrl, firstLayerDuration, singleSpanDuration)
    w.addChild(child1Ctx, tracers[4], "11", w.serviceNames[4], httpStatusCode, httpUrl, secondLayerDuration, singleSpanDuration)

    child2Ctx := w.addChild(ctx, tracers[0], "2", w.serviceNames[0], httpStatusCode, httpUrl, firstLayerDuration, singleSpanDuration)
    w.addChild(child2Ctx, tracers[7], "21", w.serviceNames[7], httpStatusCode, httpUrl, secondLayerDuration, singleSpanDuration)

    w.addChild(ctx, tracers[0], "3", w.serviceNames[0], httpStatusCode, httpUrl, firstLayerDuration, singleSpanDuration)

    for i := 0; i< 9; i++ {
        child4Ctx := w.addChild(ctx, tracers[0], "4", w.serviceNames[0], httpStatusCode, httpUrl, firstLayerDuration, singleSpanDuration)
        w.addChild(child4Ctx, tracers[4], "41", w.serviceNames[4], httpStatusCode, httpUrl, secondLayerDuration, singleSpanDuration)
    }

    child5Ctx := w.addChild(ctx, tracers[0], "5", w.serviceNames[0], httpStatusCode, httpUrl, firstLayerDuration, singleSpanDuration)
    w.addChild(child5Ctx, tracers[1], "51", w.serviceNames[1], httpStatusCode, httpUrl, secondLayerDuration, singleSpanDuration)

    limiter.Wait(context.Background())

    opt := trace.WithTimestamp(startTime.Add(traceDuration))
    sp.End(opt)

}