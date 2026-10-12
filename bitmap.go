package dataframe

type Bitmap struct{ bits []uint64 }

func NewBitmap(n int) *Bitmap { return &Bitmap{bits: make([]uint64, (n+63)/64)} }

func (b *Bitmap) Set(i int)      { b.bits[i/64] |= uint64(1) << uint(i%64) }
func (b *Bitmap) Get(i int) bool { return b != nil && b.bits[i/64]&(uint64(1)<<uint(i%64)) != 0 }
