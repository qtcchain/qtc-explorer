// qtc-explorer: block explorer for QTC backed by a Bleve index.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/qtcchain/qtc-explorer/internal/gateway"
	"github.com/qtcchain/qtc-explorer/internal/indexer"
	"github.com/qtcchain/qtc-explorer/internal/rpc"
	"github.com/qtcchain/qtc-explorer/internal/store"
	"github.com/qtcchain/qtc-explorer/internal/web"
)

func main() {
	var (
		rpcURL     = flag.String("rpc-url", envOr("QTC_RPC_URL", "http://127.0.0.1:19754"), "qtcd JSON-RPC URL")
		rpcCookie  = flag.String("rpc-cookie", os.Getenv("QTC_RPC_COOKIE"), "path to the node's .cookie file (preferred)")
		rpcUser    = flag.String("rpc-user", os.Getenv("QTC_RPC_USER"), "RPC user (if no cookie)")
		rpcPass    = flag.String("rpc-pass", os.Getenv("QTC_RPC_PASS"), "RPC password (if no cookie)")
		indexDir   = flag.String("index", envOr("QTC_EXPLORER_INDEX", "./data/index.bleve"), "Bleve index directory")
		listen     = flag.String("listen", envOr("QTC_EXPLORER_LISTEN", "127.0.0.1:8080"), "HTTP listen address")
		corsOrigin = flag.String("cors-origin", os.Getenv("QTC_EXPLORER_CORS_ORIGIN"), "exact origin allowed to call the broadcast endpoint")
		poll       = flag.Duration("poll", 2*time.Second, "node polling interval")
		halving    = flag.Int64("halving-interval", 210000, "subsidy halving interval (blocks)")
		subsidy    = flag.Int64("initial-subsidy", 50_0000_0000, "initial block subsidy in atoms")
		maxReorg   = flag.Int64("max-reorg", 100, "halt the indexer on a reorg deeper than this")
		perIP      = flag.Int("broadcast-per-minute", 6, "broadcast attempts allowed per client IP per minute")
	)
	flag.Parse()

	if err := os.MkdirAll(filepath.Dir(*indexDir), 0o750); err != nil {
		log.Fatal(err)
	}
	st, err := store.Open(*indexDir)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	nodeRPC, err := rpc.New(*rpcURL, *rpcCookie, *rpcUser, *rpcPass, 120*time.Second)
	if err != nil {
		log.Fatal(err)
	}
	gwRPC, err := rpc.New(*rpcURL, *rpcCookie, *rpcUser, *rpcPass, 30*time.Second)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	ci, err := nodeRPC.GetBlockchainInfo(ctx)
	if err != nil {
		log.Fatalf("node unreachable: %v", err)
	}
	log.Printf("connected to qtcd chain=%s height=%d", ci.Chain, ci.Blocks)

	params := indexer.Params{InitialSubsidy: *subsidy, HalvingInterval: *halving, MaxReorgDepth: *maxReorg}
	ix := indexer.New(nodeRPC, st, params, *poll)
	go ix.Run(ctx)

	gw := gateway.New(gwRPC, gateway.Config{AllowedOrigin: *corsOrigin, PerIPPerMin: *perIP})
	srv := web.New(st, nodeRPC, ix, gw, ci.Chain)

	hs := &http.Server{Addr: *listen, Handler: srv, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		sc, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = hs.Shutdown(sc)
	}()
	log.Printf("explorer listening on http://%s (chain %s)", *listen, ci.Chain)
	if err := hs.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
