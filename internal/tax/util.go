package tax

import "strconv"

func itoa(v uint) string { return strconv.FormatUint(uint64(v), 10) }
