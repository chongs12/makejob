package repository

import (
	"testing"

	"gorm.io/gorm/clause"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// TestRandomOrderExpression 验证不同数据库方言下的随机排序表达式选择正确。
func TestRandomOrderExpression(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		db       *gorm.DB
		expected string
	}{
		{
			name: "nil db defaults to postgres style",
			db:   nil,
			expected: "RANDOM()",
		},
		{
			name: "postgres uses RANDOM",
			db: &gorm.DB{
				Config: &gorm.Config{
					Dialector: testDialector{name: "postgres"},
				},
			},
			expected: "RANDOM()",
		},
		{
			name: "mysql uses RAND",
			db: &gorm.DB{
				Config: &gorm.Config{
					Dialector: testDialector{name: "mysql"},
				},
			},
			expected: "RAND()",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := randomOrderExpression(tc.db)
			if got != tc.expected {
				t.Fatalf("unexpected random order expression: got %q want %q", got, tc.expected)
			}
		})
	}
}

// testDialector 为单元测试提供最小可用的方言实现。
type testDialector struct {
	name string
}

// Name 返回测试方言名称。
func (d testDialector) Name() string {
	return d.name
}

// Initialize 满足 gorm.Dialector 接口，测试中无需实际初始化。
func (d testDialector) Initialize(*gorm.DB) error {
	return nil
}

// Migrator 满足 gorm.Dialector 接口，测试中不使用迁移器。
func (d testDialector) Migrator(*gorm.DB) gorm.Migrator {
	return nil
}

// DataTypeOf 满足 gorm.Dialector 接口，测试中无需类型映射。
func (d testDialector) DataTypeOf(*schema.Field) string {
	return ""
}

// DefaultValueOf 满足 gorm.Dialector 接口，测试中无需默认值表达式。
func (d testDialector) DefaultValueOf(*schema.Field) clause.Expression {
	return clause.Expr{}
}

// BindVarTo 满足 gorm.Dialector 接口，测试中无需变量绑定。
func (d testDialector) BindVarTo(clause.Writer, *gorm.Statement, interface{}) {
}

// QuoteTo 满足 gorm.Dialector 接口，测试中无需引用转义。
func (d testDialector) QuoteTo(clause.Writer, string) {
}

// Explain 满足 gorm.Dialector 接口，测试中直接返回 SQL。
func (d testDialector) Explain(sql string, _ ...interface{}) string {
	return sql
}
