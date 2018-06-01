// Load test the Stellar network.
package main

import (
	"context"
	"flag"
	"math"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-kit/kit/log/level"
	"github.com/laxmicoinofficial/go/build"
	"github.com/laxmicoinofficial/go/clients/orbit"
	"github.com/laxmicoinofficial/go/keypair"
	"golang.org/x/time/rate"

	//local folder structure path
	"github.com/rover/load-testing/cmd/loadtest/sequence"
	"github.com/rover/load-testing/cmd/loadtest/submitter"
)

// ClientTimeout is the orbit HTTP request timeout.
const ClientTimeout = 2 * time.Second

var (
	debugFlag             = flag.Bool("debug", false, "enable debug log level")
	orbitDomainFlag     = flag.String("address", "http://localhost:8000", "orbit address")
	stellarPassphraseFlag = flag.String("passphrase", "Rover Standlone Testnet ; April 2018", "stellar network passphrase")
	logFileFlag           = flag.String("log", "loadtest.log", "log file path")
	destinationFileFlag   = flag.String("dest", "dest.json", "destination keypairs input file")
	accountsFileFlag      = flag.String("accounts", "accounts.json", "submitter keypairs input file")
	transactionAmountFlag = flag.String("txamount", "0.00001", "transaction amount")
	opsPerTxFlag          = flag.Int("ops", 1, "amount of operations per transaction")
	testTimeLengthFlag    = flag.Int("length", 60, "test length in seconds")
	numSubmittersFlag     = flag.Int("submitters", 0, "amount of concurrent submitters; use 0 to use the number of accounts available")
	txsPerSecondFlag      = flag.Float64("rate", 10, "transaction rate limit in seconds. use 0 disable rate limiting")
	burstLimitFlag        = flag.Int("burst", 3, "burst rate limit")
	nativeAssetFlag       = flag.Bool("native", true, "set to false to use a non-native asset")
)

// Run is the main function of this application. It returns a status exit code for main().
func Run() int {
	flag.Parse()

	if *txsPerSecondFlag == 0.0 {
		*txsPerSecondFlag = math.Inf(1)
	}

	// Init logger
	logFile, err := os.OpenFile(*logFileFlag, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer logFile.Close()
	logger := InitLoggers(logFile, *debugFlag)

	// Load submitter account keypairs
	keypairs, err := InitKeypairs(*accountsFileFlag)
	if err != nil {
		level.Error(logger).Log("msg", err)
		return 1
	}

	// Load destination account keypairs
	destinations, err := InitKeypairs(*destinationFileFlag)
	if err != nil {
		level.Error(logger).Log("msg", err)
		return 1
	}

	client := orbit.Client{
		URL:  *orbitDomainFlag,
		HTTP: &http.Client{Timeout: ClientTimeout},
	}

	LogBalances(&client, keypairs, logger)

	// Init rate limiter
	limiter := rate.NewLimiter(rate.Limit(*txsPerSecondFlag), *burstLimitFlag)

	// Create top-level context. Will be sent to submitter goroutines for stopping them
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Cancel the context if not done so already when test is complete.

	network := build.Network{*stellarPassphraseFlag}

	if *numSubmittersFlag <= 0 || *numSubmittersFlag > len(keypairs) {
		*numSubmittersFlag = len(keypairs)
	}

	// Generate workers for submitting operations.
	submitters := make([]*submitter.Submitter, *numSubmittersFlag)
	sequenceProvider := sequence.New(&client, logger)
	for i := 0; i < *numSubmittersFlag; i++ {
		level.Debug(logger).Log("msg", "creating submitter", "submitter_index", i)
		submitters[i], err = submitter.New(&client, network, sequenceProvider, keypairs[i].(*keypair.Full), destinations, *transactionAmountFlag, *opsPerTxFlag)
		if err != nil {
			level.Error(logger).Log("msg", err, "submitter_index", i)
			return 1
		}
	}

	// Start transaction submission
	startTime := time.Now()
	for i := 0; i < *numSubmittersFlag; i++ {
		submitters[i].StartSubmission(ctx, limiter, logger, *nativeAssetFlag)
	}

	// Listen for OS signals
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Stop when timer is up or when a signal is caught
	select {
	case <-time.After(time.Duration(*testTimeLengthFlag) * time.Second):
		level.Info(logger).Log("msg", "test time reached")
		break
	case s := <-done:
		level.Info(logger).Log("msg", "received signal", "type", s)
		break
	}
	level.Info(logger).Log("msg", "closing")

	// Stop all submitters
	cancel()
	var wg sync.WaitGroup
	for i, s := range submitters {
		wg.Add(1)
		go func(i int, s *submitter.Submitter) {
			defer wg.Done()
			<-submitters[i].Stopped
		}(i, s)
	}
	wg.Wait()

	level.Info(logger).Log("execution_time", time.Since(startTime))

	LogBalances(&client, keypairs, logger)

	return 0
}

func main() {
	os.Exit(Run())
}
