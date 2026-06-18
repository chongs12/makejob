package data

import (
	"context"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestArchiveRepoGetWeakTopicsFiltersSoftDeleted 验证薄弱点聚合查询会显式过滤软删除记录。
func TestArchiveRepoGetWeakTopicsFiltersSoftDeleted(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer sqlDB.Close()

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 sqlDB,
		PreferSimpleProtocol: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open gorm db: %v", err)
	}

	repo := NewArchiveRepo(db)
	rows := sqlmock.NewRows([]string{"tag", "count"}).
		AddRow("grpc", 3).
		AddRow("sql", 2)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT jsonb_array_elements_text(mistake_tags::jsonb) as tag, COUNT(*) as count
			 FROM learning_archive_entries
			 WHERE user_id = $1 AND deleted_at IS NULL AND mistake_tags IS NOT NULL AND mistake_tags != ''
			 GROUP BY tag ORDER BY count DESC LIMIT $2`)).
		WithArgs(uint64(7), int32(10)).
		WillReturnRows(rows)

	topics, err := repo.GetWeakTopics(context.Background(), 7, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(topics) != 2 || topics[0] != "grpc" || topics[1] != "sql" {
		t.Fatalf("unexpected topics: %#v", topics)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
