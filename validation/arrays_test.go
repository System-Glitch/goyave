package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateArray(t *testing.T) {
	assert.True(t, validateArray("field", []string{"test"}, []string{}, map[string]interface{}{}))
	assert.True(t, validateArray("field", []int{5}, []string{}, map[string]interface{}{}))
	assert.True(t, validateArray("field", []float64{5.5}, []string{}, map[string]interface{}{}))
	assert.True(t, validateArray("field", []bool{true}, []string{}, map[string]interface{}{}))
	assert.False(t, validateArray("field", map[string]string{}, []string{}, map[string]interface{}{}))
	assert.False(t, validateArray("field", "test", []string{}, map[string]interface{}{}))
	assert.False(t, validateArray("field", 5, []string{}, map[string]interface{}{}))
	assert.False(t, validateArray("field", 5.0, []string{}, map[string]interface{}{}))
	assert.False(t, validateArray("field", true, []string{}, map[string]interface{}{}))
}

func TestValidateDistinct(t *testing.T) {
	assert.True(t, validateDistinct("field", []string{"test", "test2", "test3"}, []string{}, map[string]interface{}{}))
	assert.True(t, validateDistinct("field", []int{1, 2, 3}, []string{}, map[string]interface{}{}))
	assert.True(t, validateDistinct("field", []float64{1.2, 4.3, 2.4, 3.5, 4.5, 4.30001}, []string{}, map[string]interface{}{}))
	assert.True(t, validateDistinct("field", []bool{true, false}, []string{}, map[string]interface{}{}))

	assert.False(t, validateDistinct("field", []string{"test", "test2", "test3", "test2"}, []string{}, map[string]interface{}{}))
	assert.False(t, validateDistinct("field", []int{1, 4, 2, 3, 4}, []string{}, map[string]interface{}{}))
	assert.False(t, validateDistinct("field", []float64{1.2, 4.3, 2.4, 3.5, 4.5, 4.30001, 4.3}, []string{}, map[string]interface{}{}))

	// Not array
	assert.False(t, validateDistinct("field", 8, []string{}, map[string]interface{}{}))
	assert.False(t, validateDistinct("field", 8.0, []string{}, map[string]interface{}{}))
	assert.False(t, validateDistinct("field", "string", []string{}, map[string]interface{}{}))
}

func TestValidateIn(t *testing.T) {
	assert.True(t, validateIn("field", "dolor", []string{"lorem", "ipsum", "sit", "dolor", "amet"}, map[string]interface{}{}))
	assert.False(t, validateIn("field", "dolors", []string{"lorem", "ipsum", "sit", "dolor", "amet"}, map[string]interface{}{}))
	assert.False(t, validateIn("field", "hello world", []string{"lorem", "ipsum", "sit", "dolor", "amet"}, map[string]interface{}{}))

	assert.True(t, validateIn("field", 2.5, []string{"1", "2.4", "2.65", "87", "2.5"}, map[string]interface{}{}))
	assert.False(t, validateIn("field", 2.51, []string{"1", "2.4", "2.65", "87", "2.5"}, map[string]interface{}{}))

	assert.False(t, validateIn("field", []string{"1"}, []string{"1", "2.4", "2.65", "87", "2.5"}, map[string]interface{}{}))
	assert.Panics(t, func() { validateIn("field", "hi", []string{}, map[string]interface{}{}) })
}

func TestValidateNotIn(t *testing.T) {
	assert.False(t, validateNotIn("field", "dolor", []string{"lorem", "ipsum", "sit", "dolor", "amet"}, map[string]interface{}{}))
	assert.True(t, validateNotIn("field", "dolors", []string{"lorem", "ipsum", "sit", "dolor", "amet"}, map[string]interface{}{}))
	assert.True(t, validateNotIn("field", "hello world", []string{"lorem", "ipsum", "sit", "dolor", "amet"}, map[string]interface{}{}))

	assert.False(t, validateNotIn("field", 2.5, []string{"1", "2.4", "2.65", "87", "2.5"}, map[string]interface{}{}))
	assert.True(t, validateNotIn("field", 2.51, []string{"1", "2.4", "2.65", "87", "2.5"}, map[string]interface{}{}))

	assert.False(t, validateNotIn("field", []string{"1"}, []string{"1", "2.4", "2.65", "87", "2.5"}, map[string]interface{}{}))
	assert.Panics(t, func() { validateNotIn("field", "hi", []string{}, map[string]interface{}{}) })
}