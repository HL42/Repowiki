package wiki

import (
	"context"
	"fmt"
	"strings"
)

func (e *Engine) accumulateEntityPage(ctx context.Context, entityName, sourcePage string, facts []string) error {
	slug := sanitizeSlug(entityName)
	if slug == "" {
		return fmt.Errorf("实体名称为空")
	}

	existing, err := e.Storage.ReadPage("entities", slug)
	existingContent := ""
	if err == nil {
		existingContent = existing
	}

	newContent, err := e.LLM.AccumulateEntityPage(ctx, entityName, existingContent, sourcePage, strings.Join(facts, "\n"))
	if err != nil {
		return fmt.Errorf("LLM 合并失败: %w", err)
	}

	return e.Storage.WritePage("entities", slug, newContent)
}

func (e *Engine) accumulateConceptPage(ctx context.Context, conceptName, sourcePage string, facts []string) error {
	slug := sanitizeSlug(conceptName)
	if slug == "" {
		return fmt.Errorf("概念名称为空")
	}

	existing, err := e.Storage.ReadPage("concepts", slug)
	existingContent := ""
	if err == nil {
		existingContent = existing
	}

	newContent, err := e.LLM.AccumulateConceptPage(ctx, conceptName, existingContent, sourcePage, strings.Join(facts, "\n"))
	if err != nil {
		return fmt.Errorf("LLM 合并失败: %w", err)
	}

	return e.Storage.WritePage("concepts", slug, newContent)
}
