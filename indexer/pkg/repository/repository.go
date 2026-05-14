package repository

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

var (
	ErrGeneric error = errors.New("DBX: Internal server error")

	ErrDuplicate        error = errors.New("DBXO: Duplicate")
	ErrNotFound         error = errors.New("DBXQ: Not found")
	ErrRelationNotExist error = errors.New("DBXO: Relation not exists")
)

var (
	UniqueViolation     = "23505"
	ForeignKeyViolation = "23503"
)

type Repository[T any] interface {
	Find(ctx context.Context, options FindOptions) ([]*T, error)
	FindOne(ctx context.Context, options FindOptions) (*T, error)
	Save(ctx context.Context, entity *T) error
	Count(ctx context.Context, options FindOptions) (int64, error)
	Transaction(ctx context.Context, fn func(txRepo Repository[T]) error) error
	GetDB() *gorm.DB
}

// gorm generic repository
type repository[T any] struct {
	db *gorm.DB
}

func NewRepository[T any](db *gorm.DB) Repository[T] {
	return &repository[T]{db: db}
}

func (r *repository[T]) handleDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case UniqueViolation:
			return ErrDuplicate
		case ForeignKeyViolation:
			return ErrRelationNotExist
		default:
			return nil
		}
	}
	return nil
}

func (r *repository[T]) WrapError(ctx context.Context, err error) error {
	handled := r.handleDBError(err)
	if handled != nil {
		return handled
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return ErrGeneric
	}
	return nil
}

func (r *repository[T]) applyFindOptionsToDB(db *gorm.DB, options FindOptions) *gorm.DB {
	isSelectAll := len(options.Select) == 1 && options.Select[0] == "*"
	if options.Select != nil && !isSelectAll {
		db = db.Select(strings.Join(options.Select, ","))
	}
	if options.Where != nil {
		db = db.Where(map[string]any(options.Where))
	}
	if options.Order != nil {
		var orders string
		for field, order := range options.Order {
			orders += fmt.Sprintf("%s %s,", field, order)
		}
		orders = strings.TrimSuffix(orders, ",")
		db = db.Order(orders)
	}
	if options.Limit != 0 {
		db = db.Limit(int(options.Limit))
	}
	if options.Offset != 0 {
		db = db.Offset(int(options.Offset))
	}
	if options.Relations != nil {
		for relation, fields := range options.Relations {
			cFields := fields
			db = db.Preload(relation, func(tx *gorm.DB) *gorm.DB {
				hasID := false
				for _, field := range cFields {
					parts := strings.Split(field, ",")
					if slices.Contains(parts, "id") || field == "*" {
						hasID = true
						break
					}
				}
				if !hasID {
					cFields = append(cFields, "id")
				}
				if options.RelationFilters != nil {
					if filter, ok := options.RelationFilters[relation]; ok && filter != nil {
						tx = tx.Where(map[string]any(filter))
					}
				}
				return tx.Select([]string(cFields))
			})
		}
	}
	return db
}

func (r *repository[T]) Find(ctx context.Context, options FindOptions) ([]*T, error) {
	var results []*T
	db := r.db.WithContext(ctx).Model(results)
	db = r.applyFindOptionsToDB(db, options)
	if err := db.Find(&results).Error; err != nil {
		return results, r.WrapError(ctx, err)
	}
	return results, nil
}

// FindOne returns the first record matching the options, or ErrNotFound if none.
func (r *repository[T]) FindOne(ctx context.Context, options FindOptions) (*T, error) {
	var result T
	db := r.db.WithContext(ctx).Model(&result)
	db = r.applyFindOptionsToDB(db, options)
	if err := db.First(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, r.WrapError(ctx, err)
	}
	return &result, nil
}

// Save creates or updates an entity (upsert via GORM Save).
func (r *repository[T]) Save(ctx context.Context, entity *T) error {
	if err := r.db.WithContext(ctx).Save(entity).Error; err != nil {
		return r.WrapError(ctx, err)
	}
	return nil
}

func (r *repository[T]) Count(ctx context.Context, options FindOptions) (int64, error) {
	var count int64
	var entity T
	db := r.db.WithContext(ctx).Model(&entity)
	if options.Where != nil {
		db = db.Where(map[string]any(options.Where))
	}
	if err := db.Count(&count).Error; err != nil {
		return 0, r.WrapError(ctx, err)
	}
	return count, nil
}

func (r *repository[T]) Transaction(
	ctx context.Context,
	fn func(txRepo Repository[T]) error,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(NewRepository[T](tx))
	})
}

func (r *repository[T]) GetDB() *gorm.DB {
	return r.db
}
