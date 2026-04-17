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
}

// IsValidTransition 判断状态转换是否合法。
func (s IssueStatus) IsValidTransition(target IssueStatus) bool {
	transitions := map[IssueStatus]map[IssueStatus]bool{
		IssueStatusRegistered: {
			IssueStatusPending: true, IssueStatusProcessing: true,
			IssueStatusUnplanned: true, IssueStatusClosed: true,
		},
		IssueStatusPending: {
			IssueStatusProcessing: true, IssueStatusResolved: true,
			IssueStatusUnplanned: true, IssueStatusClosed: true,
		},
		IssueStatusProcessing: {
			IssueStatusPending: true, IssueStatusResolved: true,
			IssueStatusUnplanned: true, IssueStatusRegistered: true, IssueStatusClosed: true,
		},
		IssueStatusResolved: {
			IssueStatusClosed: true, IssueStatusRegistered: true,
		},
		IssueStatusUnplanned: {
			IssueStatusPending: true, IssueStatusProcessing: true, IssueStatusClosed: true,
		},
		IssueStatusClosed: {
			IssueStatusRegistered: true,
		},
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
