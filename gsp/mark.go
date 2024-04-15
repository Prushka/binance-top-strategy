package gsp

type MarkForRemoval struct {
	Grids map[int]float64
}

var ForRemoval = MarkForRemoval{
	Grids: make(map[int]float64),
}

func GridMarkForRemoval(gid int, maxLoss float64) {
	prev, ok := ForRemoval.Grids[gid]
	if ok && prev <= maxLoss {
		return
	}
	ForRemoval.Grids[gid] = maxLoss
}

func GetMaxLoss(gid int) *float64 {
	if v, ok := ForRemoval.Grids[gid]; ok {
		return &v
	}
	return nil
}
