package domain

import (
	"strings"
	"testing"
)

func TestNamesRejectAmbiguousOrInvalidPersonnel(t *testing.T) {
	for _, name := range []string{"", "   ", "Ana\nMaría", "\tAna", "Ana\x00", string([]byte{0xff}), strings.Repeat("á", 151), strings.Repeat(" ", 150) + "a"} {
		if _, err := NormalizeName(name); err == nil {
			t.Errorf("accepted invalid name %q", name)
		}
	}
	name, err := NormalizeName("  María Pérez  ")
	if err != nil || name != "María Pérez" {
		t.Fatalf("normalized name = %q, %v", name, err)
	}
}

func TestPaginationValidatesBoundsAndOffsetOverflow(t *testing.T) {
	for _, page := range []Page{{0, 20}, {1, 0}, {1, 101}, {-1, 20}, {int(^uint(0) >> 1), 100}} {
		if err := page.Validate(); err == nil {
			t.Errorf("accepted invalid page %+v", page)
		}
	}
	page := Page{2, 100}
	if err := page.Validate(); err != nil || page.Offset() != 100 {
		t.Fatalf("page = %+v, %v", page, err)
	}
}
