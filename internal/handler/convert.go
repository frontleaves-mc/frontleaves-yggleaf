package handler

import (
	apiIssue "github.com/frontleaves-mc/frontleaves-yggleaf/api/issue"
	apiLibrary "github.com/frontleaves-mc/frontleaves-yggleaf/api/library"
	apiUser "github.com/frontleaves-mc/frontleaves-yggleaf/api/user"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/models"
)

// skinDTOToResponse 将 SkinDTO 转换为 api/library.SkinResponse DTO。
func skinDTOToResponse(dto *models.SkinDTO) apiLibrary.SkinResponse {
	return apiLibrary.SkinResponse{
		ID:             dto.ID,
		UserID:         dto.UserID,
		Name:           dto.Name,
		TextureURL:     dto.TextureURL,
		TextureHash:    dto.TextureHash,
		Model:          dto.Model,
		IsPublic:       dto.IsPublic,
		UpdatedAt:      dto.UpdatedAt,
		AssignmentType: dto.AssignmentType,
	}
}

// skinDTOsToResponses 批量将 SkinDTO 列表转换为 SkinResponse DTO 列表。
func skinDTOsToResponses(dtos []models.SkinDTO) []apiLibrary.SkinResponse {
	responses := make([]apiLibrary.SkinResponse, len(dtos))
	for i, dto := range dtos {
		responses[i] = skinDTOToResponse(&dto)
	}
	return responses
}

// capeDTOToResponse 将 CapeDTO 转换为 api/library.CapeResponse DTO。
func capeDTOToResponse(dto *models.CapeDTO) apiLibrary.CapeResponse {
	return apiLibrary.CapeResponse{
		ID:             dto.ID,
		UserID:         dto.UserID,
		Name:           dto.Name,
		TextureURL:     dto.TextureURL,
		TextureHash:    dto.TextureHash,
		IsPublic:       dto.IsPublic,
		UpdatedAt:      dto.UpdatedAt,
		AssignmentType: dto.AssignmentType,
	}
}

// capeDTOsToResponses 批量将 CapeDTO 列表转换为 CapeResponse DTO 列表。
func capeDTOsToResponses(dtos []models.CapeDTO) []apiLibrary.CapeResponse {
	responses := make([]apiLibrary.CapeResponse, len(dtos))
	for i, dto := range dtos {
		responses[i] = capeDTOToResponse(&dto)
	}
	return responses
}

// gameProfileDTOToResponse 将 GameProfileDTO 转换为 api/user.GameProfileResponse DTO。
func gameProfileDTOToResponse(dto *models.GameProfileDTO) apiUser.GameProfileResponse {
	resp := apiUser.GameProfileResponse{
		ID:            dto.ID,
		UserID:        dto.UserID,
		UUID:          dto.UUID,
		Name:          dto.Name,
		SkinLibraryID: dto.SkinLibraryID,
		CapeLibraryID: dto.CapeLibraryID,
		UpdatedAt:     dto.UpdatedAt,
	}
	if dto.Skin != nil {
		skinResp := skinDTOToResponse(dto.Skin)
		resp.Skin = &skinResp
	}
	if dto.Cape != nil {
		capeResp := capeDTOToResponse(dto.Cape)
		resp.Cape = &capeResp
	}
	return resp
}

// gameProfileDTOsToResponses 批量将 GameProfileDTO 列表转换为 GameProfileResponse DTO 列表。
func gameProfileDTOsToResponses(dtos []models.GameProfileDTO) []apiUser.GameProfileResponse {
	responses := make([]apiUser.GameProfileResponse, len(dtos))
	for i, dto := range dtos {
		responses[i] = gameProfileDTOToResponse(&dto)
	}
	return responses
}

// skinSimpleDTOToResponse 将 SkinSimpleDTO 转换为 api/library.SkinSimpleResponse。
func skinSimpleDTOToResponse(dto models.SkinSimpleDTO) apiLibrary.SkinSimpleResponse {
	return apiLibrary.SkinSimpleResponse{
		ID:   dto.ID,
		Name: dto.Name,
	}
}

// skinSimpleDTOsToResponses 批量转换。
func skinSimpleDTOsToResponses(dtos []models.SkinSimpleDTO) []apiLibrary.SkinSimpleResponse {
	responses := make([]apiLibrary.SkinSimpleResponse, len(dtos))
	for i, dto := range dtos {
		responses[i] = skinSimpleDTOToResponse(dto)
	}
	return responses
}

// capeSimpleDTOToResponse 将 CapeSimpleDTO 转换为 api/library.CapeSimpleResponse。
func capeSimpleDTOToResponse(dto models.CapeSimpleDTO) apiLibrary.CapeSimpleResponse {
	return apiLibrary.CapeSimpleResponse{
		ID:   dto.ID,
		Name: dto.Name,
	}
}

// capeSimpleDTOsToResponses 批量转换。
func capeSimpleDTOsToResponses(dtos []models.CapeSimpleDTO) []apiLibrary.CapeSimpleResponse {
	responses := make([]apiLibrary.CapeSimpleResponse, len(dtos))
	for i, dto := range dtos {
		responses[i] = capeSimpleDTOToResponse(dto)
	}
	return responses
}

// ==================== Issue Convert Functions ====================

func issueDTOToResponse(dto *models.IssueDTO) apiIssue.IssueListItem {
	return issueDTOToListItem(dto)
}

func issueDTOToListItem(dto *models.IssueDTO) apiIssue.IssueListItem {
	return apiIssue.IssueListItem{
		Issue: apiIssue.IssueEntityWrapper{
			ID:          dto.ID,
			UserID:      dto.UserID,
			IssueTypeID: dto.IssueTypeID,
			Title:       dto.Title,
			Content:     dto.Content,
			Status:      string(dto.Status),
			Priority:    string(dto.Priority),
			AdminNote:   dto.AdminNote,
			ClosedAt:    dto.ClosedAt,
			CreatedAt:   dto.CreatedAt,
			UpdatedAt:   dto.UpdatedAt,
		},
		IssueTypeName: dto.IssueTypeName,
		ReplyCount:    dto.ReplyCount,
	}
}

func issueDetailDTOToResponse(dto *models.IssueDetailDTO) apiIssue.IssueDetailResponse {
	replies := make([]apiIssue.IssueReplyItem, len(dto.Replies))
	for i, r := range dto.Replies {
		replies[i] = apiIssue.IssueReplyItem{
			IssueReply: apiIssue.IssueReplyEntityWrapper{
				ID:           r.ID,
				IssueID:      r.IssueID,
				UserID:       r.UserID,
				Content:      r.Content,
				IsAdminReply: r.IsAdminReply,
				CreatedAt:    r.CreatedAt,
			},
			Username: r.Username,
		}
	}
	attachments := make([]apiIssue.IssueAttachmentItem, len(dto.Attachments))
	for i, a := range dto.Attachments {
		attachments[i] = apiIssue.IssueAttachmentItem{
			ID:       a.ID,
			FileName: a.FileName,
			FileSize: a.FileSize,
			MimeType: a.MimeType,
			FileURL:  a.FileURL,
		}
	}
	return apiIssue.IssueDetailResponse{
		Issue: apiIssue.IssueEntityWrapper{
			ID:          dto.Issue.ID,
			UserID:      dto.Issue.UserID,
			IssueTypeID: dto.Issue.IssueTypeID,
			Title:       dto.Issue.Title,
			Content:     dto.Issue.Content,
			Status:      string(dto.Issue.Status),
			Priority:    string(dto.Issue.Priority),
			AdminNote:   dto.Issue.AdminNote,
			ClosedAt:    dto.Issue.ClosedAt,
			CreatedAt:   dto.Issue.CreatedAt,
			UpdatedAt:   dto.Issue.UpdatedAt,
		},
		IssueType: apiIssue.IssueTypeEntityWrapper{
			ID:          dto.IssueType.ID,
			Name:        dto.IssueType.Name,
			Description: dto.IssueType.Description,
			SortOrder:   dto.IssueType.SortOrder,
			IsEnabled:   dto.IssueType.IsEnabled,
		},
		Replies:     replies,
		Attachments: attachments,
	}
}

func issueReplyDTOToResponse(dto *models.IssueReplyDTO) apiIssue.IssueReplyItem {
	return apiIssue.IssueReplyItem{
		IssueReply: apiIssue.IssueReplyEntityWrapper{
			ID:           dto.ID,
			IssueID:      dto.IssueID,
			UserID:       dto.UserID,
			Content:      dto.Content,
			IsAdminReply: dto.IsAdminReply,
			CreatedAt:    dto.CreatedAt,
		},
		Username: dto.Username,
	}
}

func issueAttachmentDTOToResponse(dto *models.IssueAttachmentDTO) apiIssue.IssueAttachmentItem {
	return apiIssue.IssueAttachmentItem{
		ID:       dto.ID,
		FileName: dto.FileName,
		FileSize: dto.FileSize,
		MimeType: dto.MimeType,
		FileURL:  dto.FileURL,
	}
}

func issueTypeDTOToResponse(dto *models.IssueTypeDTO) apiIssue.IssueTypeListItem {
	return apiIssue.IssueTypeListItem{
		ID:          dto.ID,
		Name:        dto.Name,
		Description: dto.Description,
		SortOrder:   dto.SortOrder,
	}
}
