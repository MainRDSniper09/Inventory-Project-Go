// Command inventory es el COMPOSITION ROOT de la aplicación: el único lugar
// donde se conocen las implementaciones concretas. Aquí se construyen los
// adaptadores, se inyectan en los servicios y se arranca el servidor.
//
// Cambiar SQLite por PostgreSQL, o HTTP por gRPC, sólo requeriría tocar este
// archivo y escribir el nuevo adaptador: el núcleo no cambia (Open/Closed).
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpadapter "github.com/MainRDSniper09/Inventory-Project-Go/internal/adapters/http"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/adapters/sqlite"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/services"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/platform/clock"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/platform/config"
)

// Valores inyectados en tiempo de compilación con -ldflags (ver Makefile/GoReleaser).
var (
	version = "dev"
	commit  = "none"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "consulta /healthz y termina (para Docker HEALTHCHECK)")
	showVersion := flag.Bool("version", false, "muestra la versión y termina")
	flag.Parse()

	if *showVersion {
		fmt.Printf("inventory %s (%s)\n", version, commit)
		return
	}

	cfg, err := config.Load(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(2)
	}
	if *healthcheck {
		os.Exit(runHealthcheck(cfg.Addr))
	}

	log := newLogger(cfg)
	if err := run(cfg, log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ---------- adaptadores secundarios (salida)
	db, err := sqlite.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := sqlite.Migrate(ctx, db); err != nil {
		return err
	}

	app := wire(db)

	if cfg.Seed {
		if err := seed(ctx, app); err != nil {
			return fmt.Errorf("seed: %w", err)
		}
	}

	// ---------- adaptador primario (entrada)
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpadapter.NewHandler(app, clock.System{}, log),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("server listening", "addr", cfg.Addr, "db", cfg.DBPath, "version", version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	// Apagado ordenado: dejamos terminar las peticiones en curso.
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// wire construye el grafo de dependencias (inyección manual, sin magia).
func wire(db *sql.DB) httpadapter.Services {
	clk := clock.System{}
	tx := sqlite.NewTxManager(db)
	products := sqlite.NewProductRepository(db)
	categories := sqlite.NewCategoryRepository(db)
	stock := sqlite.NewStockRepository(db)
	sales := sqlite.NewSaleRepository(db)
	reports := sqlite.NewReportRepository(db)

	return httpadapter.Services{
		Categories: services.NewCategoryService(categories, clk),
		Products:   services.NewProductService(products, categories, stock, tx, clk),
		Inventory:  services.NewInventoryService(products, stock, tx, clk),
		Sales:      services.NewSaleService(products, stock, sales, tx, clk),
		Reports:    services.NewReportService(reports),
	}
}

func newLogger(cfg config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	if cfg.LogFormat == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}

func runHealthcheck(addr string) int {
	host := addr
	if len(host) > 0 && host[0] == ':' {
		host = "127.0.0.1" + host
	}
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + host + "/healthz")
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck: status", resp.StatusCode)
		return 1
	}
	return 0
}
