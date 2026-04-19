package shared

import (
	"fmt"
	"sync/atomic"
)

// scenarioSeq — счётчик сценариев внутри текущего test-бинаря.
// Каждый per-epic пакет компилируется отдельно и начинает с 0.
var scenarioSeq atomic.Uint64

// epicPrefix — двузначный номер эпика, выставляется один раз из RunEpic.
// Добавляет визуальный префикс к маркеру сценария: 01-001 — 01_delivery,
// 02-001 — 02_filters и т.д.
var epicPrefix string

// SetEpicPrefix задаёт префикс эпика для GeneratePrefix.
// Вызывается из RunEpic до старта сюиты.
func SetEpicPrefix(prefix string) {
	epicPrefix = prefix
}

// GeneratePrefix возвращает маркер сценария в формате «NN-NNN».
func GeneratePrefix() string {
	return fmt.Sprintf("%s-%03d", epicPrefix, scenarioSeq.Add(1))
}
