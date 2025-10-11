package metric

import (
	"testing"
)

func TestOtelGaugeSet(t *testing.T) {
	// 创建一个测试gauge
	gauge := newOTelGauge("test_gauge", "Test gauge", []string{"label1", "label2"})

	// 测试Set方法
	g1 := gauge.With("value1", "value2")
	g1.Set(100.0)

	// 验证内部状态
	key := g1.(*otelGauge).makeAttrKey()
	if value, exists := g1.(*otelGauge).values.Load(key); !exists || value.(float64) != 100.0 {
		t.Errorf("Expected value 100.0, got %v", value)
	}

	// 测试再次Set
	g1.Set(150.0)
	if value, exists := g1.(*otelGauge).values.Load(key); !exists || value.(float64) != 150.0 {
		t.Errorf("Expected value 150.0, got %v", value)
	}

	// 测试不同标签组合
	g2 := gauge.With("value3", "value4")
	g2.Set(200.0)

	key2 := g2.(*otelGauge).makeAttrKey()
	if value, exists := g2.(*otelGauge).values.Load(key2); !exists || value.(float64) != 200.0 {
		t.Errorf("Expected value 200.0 for second gauge, got %v", value)
	}

	// 验证第一个gauge的值没有改变
	if value, exists := g1.(*otelGauge).values.Load(key); !exists || value.(float64) != 150.0 {
		t.Errorf("Expected first gauge to still be 150.0, got %v", value)
	}
}

func TestOtelGaugeAdd(t *testing.T) {
	gauge := newOTelGauge("test_gauge_add", "Test gauge add", []string{"label1"})

	g := gauge.With("test")

	// 测试Add
	g.Add(50.0)
	key := g.(*otelGauge).makeAttrKey()
	if value, exists := g.(*otelGauge).values.Load(key); !exists || value.(float64) != 50.0 {
		t.Errorf("Expected value 50.0 after Add, got %v", value)
	}

	// 再次Add
	g.Add(30.0)
	if value, exists := g.(*otelGauge).values.Load(key); !exists || value.(float64) != 80.0 {
		t.Errorf("Expected value 80.0 after second Add, got %v", value)
	}

	// 测试Sub
	g.Sub(20.0)
	if value, exists := g.(*otelGauge).values.Load(key); !exists || value.(float64) != 60.0 {
		t.Errorf("Expected value 60.0 after Sub, got %v", value)
	}
}

func TestOtelGaugeSetAddMixed(t *testing.T) {
	gauge := newOTelGauge("test_gauge_mixed", "Test gauge mixed operations", []string{"operation"})

	g := gauge.With("mixed_test")

	// Set初始值
	g.Set(100.0)
	key := g.(*otelGauge).makeAttrKey()
	if value, exists := g.(*otelGauge).values.Load(key); !exists || value.(float64) != 100.0 {
		t.Errorf("Expected value 100.0 after Set, got %v", value)
	}

	// Add一些值
	g.Add(25.0)
	if value, exists := g.(*otelGauge).values.Load(key); !exists || value.(float64) != 125.0 {
		t.Errorf("Expected value 125.0 after Add, got %v", value)
	}

	// 重新Set
	g.Set(200.0)
	if value, exists := g.(*otelGauge).values.Load(key); !exists || value.(float64) != 200.0 {
		t.Errorf("Expected value 200.0 after second Set, got %v", value)
	}

	// Sub一些值
	g.Sub(50.0)
	if value, exists := g.(*otelGauge).values.Load(key); !exists || value.(float64) != 150.0 {
		t.Errorf("Expected value 150.0 after Sub, got %v", value)
	}
}

func TestOtelGaugeKeyGeneration(t *testing.T) {
	gauge := newOTelGauge("test_gauge_key", "Test gauge key generation", []string{"label1", "label2"})

	// 测试相同标签值生成相同key
	g1 := gauge.With("value1", "value2")
	g2 := gauge.With("value1", "value2")

	key1 := g1.(*otelGauge).makeAttrKey()
	key2 := g2.(*otelGauge).makeAttrKey()

	if key1 != key2 {
		t.Errorf("Expected same keys for same label values, got %s and %s", key1, key2)
	}

	// 测试不同标签值生成不同key
	g3 := gauge.With("value3", "value4")
	key3 := g3.(*otelGauge).makeAttrKey()

	if key1 == key3 {
		t.Errorf("Expected different keys for different label values, but got same key %s", key1)
	}

	// 测试无标签的情况
	g4 := gauge.With()
	key4 := g4.(*otelGauge).makeAttrKey()

	if key4 != "_default_" {
		t.Errorf("Expected default key '_default_' for no labels, got %s", key4)
	}
}
