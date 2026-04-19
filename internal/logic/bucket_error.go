package logic

import (
	"context"
	"strings"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	"connectrpc.com/connect"
)

// mapBucketError 将 beacon-bucket SDK 返回的 connect-go 错误动态映射为对应的业务错误码。
//
// 客户端类错误（invalid_argument、not_found、permission_denied 等）映射到 4xx，
// 服务端类错误（internal、unavailable 等）映射到 5xx。
func mapBucketError(ctx context.Context, message string, err error) *xError.Error {
	msg := xError.ErrMessage(message)

	if connectErr := new(connect.Error); errorAs(err, &connectErr) {
		switch connectErr.Code() {
		case connect.CodeInvalidArgument:
			return xError.NewError(ctx, xError.FormatError, msg, true, err)
		case connect.CodeNotFound:
			return xError.NewError(ctx, xError.ResourceNotFound, msg, true, err)
		case connect.CodeAlreadyExists:
			return xError.NewError(ctx, xError.DataConflict, msg, true, err)
		case connect.CodePermissionDenied:
			return xError.NewError(ctx, xError.PermissionDenied, msg, true, err)
		case connect.CodeResourceExhausted:
			return xError.NewError(ctx, xError.ResourceExhausted, msg, true, err)
		case connect.CodeFailedPrecondition:
			return xError.NewError(ctx, xError.OperationDenied, msg, true, err)
		case connect.CodeOutOfRange:
			return xError.NewError(ctx, xError.DataOutOfRange, msg, true, err)
		case connect.CodeUnauthenticated:
			return xError.NewError(ctx, xError.Unauthorized, msg, true, err)
		case connect.CodeUnavailable:
			return xError.NewError(ctx, xError.ServiceUnavailable, msg, true, err)
		case connect.CodeDeadlineExceeded:
			return xError.NewError(ctx, xError.Timeout, msg, true, err)
		case connect.CodeCanceled:
			return xError.NewError(ctx, xError.OperationError, msg, true, err)
		case connect.CodeAborted:
			return xError.NewError(ctx, xError.OperationFailed, msg, true, err)
		case connect.CodeUnimplemented:
			return xError.NewError(ctx, xError.UnsupportedOp, msg, true, err)
		}
	}

	// 降级：通过错误消息前缀匹配
	errMsg := err.Error()
	clientPrefixes := []string{
		"invalid_argument",
		"not_found",
		"already_exists",
		"permission_denied",
		"resource_exhausted",
		"failed_precondition",
		"out_of_range",
		"unauthenticated",
	}
	for _, prefix := range clientPrefixes {
		if strings.HasPrefix(errMsg, prefix) {
			return xError.NewError(ctx, xError.OperationDenied, msg, true, err)
		}
	}

	return xError.NewError(ctx, xError.ServerInternalError, msg, true, err)
}

// errorAs 从错误链中提取 connect.Error。
func errorAs(err error, target **connect.Error) bool {
	if ce, ok := err.(*connect.Error); ok {
		*target = ce
		return true
	}
	unwrapped := err
	for unwrapped != nil {
		if ce, ok := unwrapped.(*connect.Error); ok {
			*target = ce
			return true
		}
		if u, ok := unwrapped.(interface{ Unwrap() error }); ok {
			unwrapped = u.Unwrap()
		} else {
			break
		}
	}
	return false
}
