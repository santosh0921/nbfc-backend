package risk

import "strconv"

func itoa(v uint) string  { return strconv.FormatUint(uint64(v), 10) }
func itoaInt(v int) string { return strconv.Itoa(v) }
