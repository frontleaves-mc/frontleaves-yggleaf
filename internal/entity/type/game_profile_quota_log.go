package entityType

type ObType struct {
	Name string // ObType 操作类型，表示游戏档案配额日志的操作类型
	Type uint8  // Type 代表配额的增减模式； 0 代表增加，1 代表减少
}

var (
	ObTypeAddGameProfile = ObType{Name: "ADD_GAME_PROFILE", Type: 1}
)

var gameProfileQuotaLogObTypeSet = map[ObType]string{
	ObTypeAddGameProfile: ObTypeAddGameProfile.Name,
}

func (t ObType) String() string {
	return t.Name
}

func (t ObType) IsValid() bool {
	_, ok := gameProfileQuotaLogObTypeSet[t]
	return ok
}

func (t ObType) getType() uint8 {
	return t.Type
}
