package gsp

type SDCount map[string]map[string]int

func (sdCount SDCount) GetSDCount(symbol, direction string) int {
	if _, ok := sdCount[symbol]; !ok {
		return 0
	}
	return sdCount[symbol][direction]
}
