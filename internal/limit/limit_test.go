package limit

import "testing"

func TestSemaphore(t *testing.T) {
	s := NewSemaphore(2)
	if !s.TryAcquire() || !s.TryAcquire() {
		t.Fatal("dua slot pertama seharusnya berhasil")
	}
	if s.TryAcquire() {
		t.Fatal("slot ketiga seharusnya ditolak")
	}
	s.Release()
	if !s.TryAcquire() {
		t.Fatal("setelah Release seharusnya berhasil lagi")
	}
}

func TestIPLimiter(t *testing.T) {
	l := NewIPLimiter(1, 3) // burst 3, isi ulang sangat lambat
	for i := 0; i < 3; i++ {
		if !l.Allow("1.2.3.4") {
			t.Fatalf("permintaan ke-%d seharusnya lolos", i+1)
		}
	}
	if l.Allow("1.2.3.4") {
		t.Error("permintaan ke-4 seharusnya ditolak")
	}
	if !l.Allow("5.6.7.8") {
		t.Error("IP lain tidak boleh ikut terkena batas")
	}
}
