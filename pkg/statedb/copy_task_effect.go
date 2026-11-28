package statedb

import (
	"context"
	"fmt"
	"slices"

	"gorm.io/gorm"
)

// Only the receipt table is optional in a pre-receipt source database.
func copyStateSourceModels(ctx context.Context, src *gorm.DB) ([]any, error) {
	src = src.WithContext(ctx)
	tables, err := src.Migrator().GetTables()
	if err != nil {
		return nil, fmt.Errorf("list source tables: %w", err)
	}
	const effects = "app_task_effects"
	present := slices.Contains(tables, effects)
	if !present && src.Name() == string(DBTypePostgres) {
		// information_schema hides tables the source role cannot read.
		if err := src.Raw("SELECT to_regclass(?) IS NOT NULL", effects).Row().Scan(&present); err != nil {
			return nil, fmt.Errorf("check source receipt table: %w", err)
		}
	}
	if present {
		return StatefulDBModels, nil
	}
	models := make([]any, 0, len(StatefulDBModels)-1)
	for _, m := range StatefulDBModels {
		if _, receiptTable := m.(AppTaskEffect); !receiptTable {
			models = append(models, m)
		}
	}
	return models, nil
}
