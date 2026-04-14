package entityType

// AssignmentType 资源关联类型，标识用户与资源的关联方式及配额影响规则。
type AssignmentType uint8

const (
	// AssignmentTypeNormal 用户自主上传的资源，计入配额消耗。
	AssignmentTypeNormal AssignmentType = 1

	// AssignmentTypeGift 管理员赠送的资源，不计入配额消耗。
	AssignmentTypeGift AssignmentType = 2

	// AssignmentTypeAdmin 系统预置/管理员分配的资源，不计入配额消耗。
	AssignmentTypeAdmin AssignmentType = 3
)

var assignmentTypeSet = map[AssignmentType]string{
	AssignmentTypeNormal: "NORMAL",
	AssignmentTypeGift:   "GIFT",
	AssignmentTypeAdmin:  "ADMIN",
}

// String 返回关联类型的字符串表示。
func (t AssignmentType) String() string {
	if s, ok := assignmentTypeSet[t]; ok {
		return s
	}
	return "UNKNOWN"
}

// IsValid 校验关联类型是否为合法值。
func (t AssignmentType) IsValid() bool {
	_, ok := assignmentTypeSet[t]
	return ok
}

// CountsTowardQuota 判断该类型是否计入用户配额。
// 只有 AssignmentTypeNormal 类型的关联才计入配额消耗。
func (t AssignmentType) CountsTowardQuota() bool {
	return t == AssignmentTypeNormal
}
