package main

import (
	"fmt"
	"github.com/niftynei/glightning/glightning"
	"github.com/niftynei/glightning/jrpc2"
	"log"
	"os"
	"strconv"
	"math/rand"
	"time"
	"encoding/hex"
	"regexp"
)

const (
	Frequency string = "sitzprobe-freq"
	Amount string = "sitzprobe-amt"
	DefaultFreq int = 60
	DefaultAmt uint64 = uint64(1)
)

// Note: if you add a new type that's not of type 'failure', you should
// update countFailures to account for this.
const (
	Success string = "success" // should never happen
	NoActiveChannelFound string = "no_active_channel_found"
	ChannelsUnavailable string = "channels_unavailable"
	NoRouteFound string = "no_route_found"
	SendPayFailed string = "sendpay_call_failed"
	Run string = "runs_started"
	UnknownError string = "unknown_error"
)

// The goal of sitzprobe is to provide a utility
// maintain a healthy node graph. The goal is to pick a
// node to pay, and then send a payment.
//
// I'd like to keep a log of what payments are successful
// and which are not successful.
var ln *glightning.Lightning
var plugin *glightning.Plugin
var reportSet map[string]uint64
var start string

func main() {
	plugin = glightning.NewPlugin(onInit)
	ln = glightning.NewLightning()
	reportSet = make(map[string]uint64)

	registerOptions()
	registerMethods()

	err := plugin.Start(os.Stdin, os.Stdout)
	if err != nil {
		log.Fatal(err)
	}
}

type Report struct {
}

type ReportResult struct {
	Frequency string `json:"frequency"`
	StartedAt string `json:"started_at"`
	Runs uint64	`json:"runs"`
	Successes uint64 `json:"successes"`
	Failures uint64  `json:"failures"`
	Stats map[string]uint64 `json:"stats"`
}

func (r *Report) New() interface{} {
	return &Report{}
}

func (r *Report) Name() string {
	return "sitzprobe-report"
}

func (r *Report) Call() (jrpc2.Result, error) {
	return &ReportResult{
		Frequency: fmt.Sprintf("every %d min", freq),
		StartedAt: start,
		Runs: reportSet[Run],
		Successes: reportSet[Success],
		Failures: countFailures(),
		Stats: reportSet,
	}, nil
}

func countFailures() uint64 {
	var result uint64
	for k,v := range reportSet {
		switch (k) {
		case Success, Run:
			// do nothing
		default:
			result += v
		}
	}
	return result
}

// Sitzprobe offers two options: 
// - frequency: repetition frequency with which Sitzprobe tries a new payment, in minutes.
// - probe amount: amount to probe with, in millisatoshis.
func registerOptions() {
	freqOption := glightning.NewOption(Frequency, "Interval to run sitzprobe on, in minutes", fmt.Sprintf("%d",DefaultFreq))

	probeamount := glightning.NewOption(Amount, "Amount to probe with, in millisatoshis", fmt.Sprintf("%d",DefaultAmt))
	plugin.RegisterOption(freqOption)
	plugin.RegisterOption(probeamount)
}

func registerMethods() {
	reportMethod := glightning.NewRpcMethod(&Report{}, "Print a probe report")
	reportMethod.LongDesc = "Returns a set of metrics around probes, including failures and successes"
	plugin.RegisterMethod(reportMethod)
}

var freq int
var amount uint64

func onInit(plugin *glightning.Plugin, options map[string]string, config *glightning.Config) {
	amount = parseAmount(options[Amount])
	freq = parseFreq(options[Frequency])

	ln.StartUp(config.RpcFile, config.LightningDir)
	start = time.Now().UTC().Format("2006-01-02T15:04:05-0700")
	reschedule(0)
}

func reschedule(number int) {
	if number != 0 {
		timer := time.NewTimer(time.Duration(freq) * time.Minute)
		// wait for timer to elapse
		<-timer.C
	}
	go run(number+1, amount)
}

func count(errType string) {
	reportSet[errType] += 1
}

func run(runNumber int, amount uint64) {
	// we wait to reschedule until after this payment
	// has successfully been 'finished'
	defer reschedule(runNumber)
	count(Run)

	// list all the available channels
	channels, err := ln.ListChannels()
	if err != nil {
		log.Printf("(RUN%d)Unable to fetch channel list: %s", runNumber, err.Error())
		count(ChannelsUnavailable)
		return
	}

	// pick a node at random from the channel list
	rand.Seed(time.Now().Unix())
	var channel glightning.Channel
	ok := false
	// try to find an active channel
	for i := 0; i < 1000 && !ok; i++ {
		n := rand.Int() % len(channels)
		// use the destination of the channel
		channel = channels[n]
		if channel.IsActive {
			ok = true
		}
	}
	if !ok {
		log.Printf("(RUN%d)Unable to find active channel out of %d channels", runNumber, len(channels))
		count(NoActiveChannelFound)
		return
	}

	// find a route to the selected node.
	route, err := ln.GetRouteSimple(channel.Destination, amount, 5)
	if err != nil {
		log.Printf("(RUN%d)Unable to find route to node id %s: %s", runNumber, channel.Destination, err.Error())
		count(NoRouteFound)
		return
	}

	// generate a fake payment hash
	fakeHash := randomPayHash()
	_, err = ln.SendPayLite(route, fakeHash)
	if err != nil {
		log.Printf("(RUN%d)Unable to send payment along route: %s", runNumber, err.Error())
		count(SendPayFailed)
		return
	}

	// block until we get a result (should fail)
	payment, err := ln.WaitSendPay(fakeHash, 0)
	if err != nil {
		log.Printf("(RUN%d)Payment successfully failed: %s", runNumber, err.Error())
		logFailure(err.Error())
	} else {
		log.Printf("(RUN%d)Payment somehow miraculously succeeded wtf. Peer %s reached", runNumber, payment.Destination)
		count(Success)
	}
}

var pattern *regexp.Regexp = regexp.MustCompile("[A-Z_]+")

func logFailure(errMsg string) {
	errorType := pattern.FindString(errMsg)
	switch errorType {
	case "":
		count(UnknownError)
	case "WIRE_INCORRECT_OR_UNKNOWN_PAYMENT_DETAILS":
		count(Success)
	default:
		count(errorType)
	}

}

func randomPayHash() string {
	b := make([]byte, 32)
	for i := range b {
		// was seeded earlier
		b[i] = byte(rand.Intn(127))
	}
	return hex.EncodeToString(b)
}

func parseAmount(setAmount string) uint64 {
	amt, err := strconv.ParseUint(setAmount, 10, 64)
	if err != nil {
		log.Printf("Invalid amount set (%s), defaulting to %d", setAmount, DefaultAmt)
		return DefaultAmt
	}
	return amt
}

func parseFreq(setFreq string) int {
	freq, err := strconv.Atoi(setFreq)
	if err != nil {
		log.Printf("Invalid frequency set (%s), defaulting to %d", setFreq, DefaultFreq)
		return DefaultFreq
	}
	if freq <= 0 {
		log.Printf("Invalid frequency set (%s), defaulting to %d", freq, DefaultFreq)
		return DefaultFreq
	}
	return freq
}
