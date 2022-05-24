package reef

var (
	metadata *string = new(string)
	lastUpdateTime uint64
	mustUpdateGap uint64 = 86400 // 1 day
)