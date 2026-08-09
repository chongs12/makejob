package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// InitMeterProvider 初始化 OTel MeterProvider。
//
// 设计说明（对应决策 2：trace 与 metrics 管线拆分）：
// 本项目 metrics 走 Prometheus 直接 scrape /metrics 端点，不通过 OTLP 导出：
//   - gRPC 指标由 go-grpc-prometheus 产出（注册到 prometheus.DefaultRegisterer）
//   - AI/HTTP 业务指标由 prometheus/client_golang 自写
//
// 因此 OTel MeterProvider 初始化为无 Reader 的实例（等同 noop：可安全调用 OTel Meter API，
// 但不收集/导出指标）。显式 otel.SetMeterProvider 避免使用 nil 全局。
//
// 如未来需要用 OTel Metrics API 埋点并暴露，可在此处替换为
// go.opentelemetry.io/otel/exporters/prometheus 导出器，注册到同一个 Registerer。
func InitMeterProvider(serviceName string) (*sdkmetric.MeterProvider, func(), error) {
	// 无 Reader -> 不导出任何指标，仅满足 MeterProvider 非 nil 约束。
	provider := sdkmetric.NewMeterProvider()
	otel.SetMeterProvider(provider)

	cleanup := func() {
		// shutdown 释放可能存在的内部资源（noop 情况下为空操作）。
		if err := provider.Shutdown(context.Background()); err != nil {
			fmt.Printf("telemetry: meter provider shutdown: %v\n", err)
		}
	}
	return provider, cleanup, nil
}
