package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"stage-rigging-release/internal/application"
	"stage-rigging-release/internal/httpapi"
	"stage-rigging-release/internal/store"
	"stage-rigging-release/internal/webui"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		log.Printf("服务退出: %v", err)
		os.Exit(1)
	}
}

func run() error {
	flags := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	addrFlag := flags.String("addr", defaultAddress, "回环监听地址")
	dbPath := flags.String("db", "rigging-release.db", "SQLite 数据库路径")
	selfcheck := flags.Bool("selfcheck", false, "运行有界主流程自检后退出")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return err
	}
	addrWasSet := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "addr" {
			addrWasSet = true
		}
	})
	addr, err := resolveAddress(*addrFlag, addrWasSet)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", addr, err)
	}
	defer listener.Close()
	databasePath := *dbPath
	if *selfcheck {
		databasePath = "file:selfcheck?mode=memory&cache=shared"
	}
	repo, err := store.Open(context.Background(), databasePath)
	if err != nil {
		return err
	}
	defer repo.Close()
	service := application.NewService(repo)
	server := &http.Server{Handler: httpapi.New(service, webui.Handler()), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	serveDone := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if err == http.ErrServerClosed {
			err = nil
		}
		serveDone <- err
	}()
	actualAddr := listener.Addr().String()
	if *selfcheck {
		checkCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		err = runSelfcheck(checkCtx, "http://"+actualAddr)
		shutdownCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		shutdownErr := server.Shutdown(shutdownCtx)
		serveErr := <-serveDone
		if err != nil {
			return err
		}
		if shutdownErr != nil {
			return shutdownErr
		}
		if serveErr != nil {
			return serveErr
		}
		fmt.Printf("selfcheck 通过，监听地址 %s，完整放行流程与摘要核验成功\n", actualAddr)
		return nil
	}
	log.Printf("舞台吊挂放行工作台已启动: http://%s", actualAddr)
	signalCtx, stopSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignal()
	select {
	case err = <-serveDone:
		return err
	case <-signalCtx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err = server.Shutdown(shutdownCtx); err != nil {
		return err
	}
	return <-serveDone
}
