package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	xEnv "github.com/bamboo-services/bamboo-base-go/env"
	xLog "github.com/bamboo-services/bamboo-base-go/log"
	xReg "github.com/bamboo-services/bamboo-base-go/register"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/app/route"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/app/startup"
)

func main() {
	reg := xReg.Register()
	start := startup.Register(reg)
	log := xLog.WithName(xLog.NamedMAIN)

	// 创建上下文和取消函数
	ctx, cancel := context.WithCancel(reg.Context)
	defer cancel()

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 注册路由器
	route.NewRoute(reg, start, reg.Context)

	// 创建 HttpServer
	getHost := xEnv.GetEnvString(xEnv.Host, "localhost")
	getPort := xEnv.GetEnvString(xEnv.Port, "5566")
	server := &http.Server{
		Addr:    getHost + ":" + getPort, // 使用配置文件中指定的端口
		Handler: reg.Serve,
	}

	// =============
	//   协程启动
	// =============
	engineSync := sync.WaitGroup{}
	engineSync.Add(1) // HTTP Server

	// 启动 HTTP 服务
	go func() {
		defer engineSync.Done()
		log.Info(reg.Context, "服务器已成功启动", slog.String("addr", "http(s)://"+server.Addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error(reg.Context, err.Error())
		}
	}()

	// 监听关闭信号（只有一个信号监听器！）
	go func() {
		<-sigChan
		cancel() // 取消上下文，触发所有组件的优雅关闭
		log.Warn(reg.Context, "正在关闭 HTTP 服务器...")
		if err := server.Shutdown(ctx); err != nil {
			log.Error(reg.Context, err.Error())
		}
	}()

	engineSync.Wait()
	log.Info(reg.Context, "所有服务已安全退出")
	return
}
