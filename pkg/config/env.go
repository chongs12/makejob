package config

import (
	"os"
	"reflect"
	"strings"
)

// ApplyEnvOverrides 遍历 v 的所有字符串字段（含嵌套指针/结构），
// 将 `${ENV_VAR}` 格式的值替换为 os.Getenv(ENV_VAR)。
//
// 用于 K8s 部署时 config.yaml 里的敏感字段用环境变量占位（方案 A 的通用形式）：
//
//	db:
//	  database:
//	    source: ${DB_SOURCE}   # K8s 用 Secret + env 注入 DB_SOURCE
//	jwt:
//	  secret: ${JWT_SECRET}
//
// 本地 dev 的 config.yaml 直接写明文值（不含 ${}），本函数不做任何改动，零影响。
// 对应 docs/observability-k8s-checklist.md 4.1 方案 A。
func ApplyEnvOverrides(v interface{}) {
	applyEnvOverrides(reflect.ValueOf(v))
}

func applyEnvOverrides(v reflect.Value) {
	if !v.IsValid() {
		return
	}
	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			return
		}
		applyEnvOverrides(v.Elem())
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			applyEnvOverrides(v.Field(i))
		}
	case reflect.String:
		s := v.String()
		if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
			envKey := s[2 : len(s)-1]
			if val := os.Getenv(envKey); val != "" {
				v.SetString(val)
			}
		}
	}
}
