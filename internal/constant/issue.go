package bConst

// ====================
//   Issue Status Constants
// ====================

// IssueStatus 问题状态常量
type IssueStatus string

const (
	IssueStatusRegistered IssueStatus = "registered" // 已登记
	IssueStatusPending    IssueStatus = "pending"    // 待处理
	IssueStatusProcessing IssueStatus = "processing" // 处理中
	IssueStatusResolved   IssueStatus = "resolved"   // 已处理
	IssueStatusUnplanned  IssueStatus = "unplanned"  // 未计划
	IssueStatusClosed     IssueStatus = "closed"     // 已关闭
)

// ValidStatuses 所有合法状态值（用于 binding:oneof 校验）
var ValidStatuses = []string{
	string(IssueStatusRegistered),
	string(IssueStatusPending),
	string(IssueStatusProcessing),
	string(IssueStatusResolved),
	string(IssueStatusUnplanned),
	string(IssueStatusClosed),
	"nofinal", // 虚拟筛选值：排除所有终态（resolved/unplanned/closed）
}

// FinalStatuses 终态列表（已处理/未计划/已关闭），用于 nofinal 筛选。
var FinalStatuses = []IssueStatus{
	IssueStatusResolved,
	IssueStatusUnplanned,
	IssueStatusClosed,
}

// IsValidTransition 判断状态转换是否合法。
func (s IssueStatus) IsValidTransition(target IssueStatus) bool {
	transitions := map[IssueStatus]map[IssueStatus]bool{
		IssueStatusRegistered: {
			IssueStatusPending:    true, // G1 → G2
			IssueStatusProcessing: true, // G1 → G2
			IssueStatusUnplanned:  true, // 特殊：未开始可直接标记无计划
			IssueStatusClosed:     true, // G1 → G3
		},
		IssueStatusPending: {
			IssueStatusProcessing: true, // G2 组内互切
			IssueStatusResolved:   true, // G2 → G3
			IssueStatusClosed:     true, // G2 → G3
		},
		IssueStatusProcessing: {
			IssueStatusPending:  true, // G2 组内互切
			IssueStatusResolved: true, // G2 → G3
			IssueStatusClosed:   true, // G2 → G3
		},
		IssueStatusResolved: {
			IssueStatusClosed: true, // G3 → G3
		},
		IssueStatusUnplanned: {
			IssueStatusClosed: true, // G3 → G3
		},
		IssueStatusClosed: {}, // 终态，不允许任何转出
	}
	if targets, ok := transitions[s]; ok {
		return targets[target]
	}
	return false
}

// ====================
//   Issue Priority Constants
// ====================

// IssuePriority 优先级常量
type IssuePriority string

const (
	PriorityLow    IssuePriority = "low"     // 低
	PriorityMedium IssuePriority = "medium"  // 中
	PriorityHigh   IssuePriority = "high"    // 高
	PriorityUrgent IssuePriority = "urgent"  // 紧急
)

// ValidPriorities 所有合法优先级值
var ValidPriorities = []string{
	string(PriorityLow), string(PriorityMedium),
	string(PriorityHigh), string(PriorityUrgent),
}
