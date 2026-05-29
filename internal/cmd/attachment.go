// Package cmd 中的 attachment.go 实现了附件和图片管理命令
package cmd

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

var (
	flagAttachmentEntryID string
	flagAttachmentType    string
	flagImagePath         string
	flagAttachmentID      string
	flagAttachmentFile    string
	flagAttachmentField   string
)

// attachmentCmd 是 attachment 父命令
var attachmentCmd = &cobra.Command{
	Use:   "attachment",
	Short: "附件管理",
}

var attachmentListCmd = &cobra.Command{
	Use:   "list",
	Short: "查询条目的附件列表",
	RunE:  runAttachmentList,
}

var attachmentDownloadCmd = &cobra.Command{
	Use:   "download",
	Short: "获取附件下载链接",
	RunE:  runAttachmentDownload,
}

var attachmentUploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "上传附件",
	RunE:  runAttachmentUpload,
}

// imageCmd 是 image 父命令
var imageCmd = &cobra.Command{
	Use:   "image",
	Short: "图片管理",
}

var imageGetCmd = &cobra.Command{
	Use:   "get",
	Short: "获取图片下载链接",
	RunE:  runImageGet,
}

// imageUploadCmd 以 base64 方式上传图片
var imageUploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "以 base64 方式上传图片",
	RunE:  runImageUpload,
}

func init() {
	// attachment list 子命令
	attachmentListCmd.Flags().StringVar(&flagAttachmentEntryID, "entry-id", "", "条目 ID（必需）")
	attachmentListCmd.Flags().StringVar(&flagAttachmentType, "type", "", "条目类型（story|bug|task）")
	attachmentListCmd.Flags().IntVar(&flagLimit, "limit", 10, "返回数量限制")
	attachmentListCmd.Flags().IntVar(&flagPage, "page", 1, "页码")
	attachmentListCmd.Flags().StringArrayVar(&flagFilter, "filter", nil, filterFlagDesc)

	// image get 子命令
	imageGetCmd.Flags().StringVar(&flagImagePath, "image-path", "", "图片路径（必需，从条目描述中获取）")

	// attachment download 子命令
	attachmentDownloadCmd.Flags().StringVar(&flagAttachmentID, "id", "", "附件 ID（必需）")

	// attachment upload 子命令
	attachmentUploadCmd.Flags().StringVar(&flagAttachmentEntryID, "entry-id", "", "条目 ID（必需）")
	attachmentUploadCmd.Flags().StringVar(&flagAttachmentType, "type", "", "条目类型（如 story_custom_field，必需）")
	attachmentUploadCmd.Flags().StringVar(&flagAttachmentField, "custom-field", "", "自定义字段英文名（必需）")
	attachmentUploadCmd.Flags().StringVar(&flagAttachmentFile, "file", "", "本地文件路径（必需）")

	attachmentCmd.AddCommand(attachmentListCmd, attachmentDownloadCmd, attachmentUploadCmd)
	rootCmd.AddCommand(attachmentCmd)

	// image upload 子命令
	imageUploadCmd.Flags().StringVar(&flagAttachmentEntryID, "entry-id", "", "条目 ID（必需）")
	imageUploadCmd.Flags().StringVar(&flagAttachmentField, "custom-field", "", "自定义字段英文名（必需）")
	imageUploadCmd.Flags().StringVar(&flagAttachmentFile, "file", "", "本地图片文件路径（必需，≤15MB）")

	imageCmd.AddCommand(imageGetCmd, imageUploadCmd)
	rootCmd.AddCommand(imageCmd)
}

func runAttachmentList(cmd *cobra.Command, args []string) error {
	if flagAttachmentEntryID == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--entry-id is required",
			"Usage: tapd attachment list --entry-id <id>")
		os.Exit(output.ExitParamError)
		return nil
	}

	req := &model.GetAttachmentsRequest{
		WorkspaceID: flagWorkspaceID,
		EntryID:     flagAttachmentEntryID,
		Type:        flagAttachmentType,
		Limit:       flagLimit,
		Page:        flagPage,
	}

	attachments, err := listWithFilters[model.Attachment](cmdContext(cmd), apiClient, "/attachments", req.ToParams(), flagFilter, "Attachment")
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, attachments, !flagPretty)
}

func runAttachmentDownload(cmd *cobra.Command, args []string) error {
	if flagAttachmentID == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--id is required",
			"Usage: tapd attachment download --id <attachment_id>")
		os.Exit(output.ExitParamError)
		return nil
	}

	req := &model.DownloadAttachmentRequest{
		WorkspaceID: flagWorkspaceID,
		ID:          flagAttachmentID,
	}
	att, err := apiClient.DownloadAttachment(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, att, !flagPretty)
}

func runAttachmentUpload(cmd *cobra.Command, args []string) error {
	if flagAttachmentEntryID == "" || flagAttachmentType == "" || flagAttachmentField == "" || flagAttachmentFile == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--entry-id, --type, --custom-field, and --file are required",
			"Usage: tapd attachment upload --entry-id <id> --type <type> --custom-field <field> --file <path>")
		os.Exit(output.ExitParamError)
		return nil
	}

	f, err := os.Open(flagAttachmentFile)
	if err != nil {
		output.PrintError(os.Stderr, "file_error", err.Error(), "")
		os.Exit(output.ExitParamError)
		return nil
	}
	defer f.Close()

	req := &model.UploadAttachmentRequest{
		WorkspaceID: flagWorkspaceID,
		Type:        flagAttachmentType,
		CustomField: flagAttachmentField,
		EntryID:     flagAttachmentEntryID,
	}
	att, err := apiClient.UploadAttachment(context.Background(), req, filepath.Base(flagAttachmentFile), f)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, att, !flagPretty)
}

func runImageGet(cmd *cobra.Command, args []string) error {
	if flagImagePath == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--image-path is required",
			"Usage: tapd image get --image-path <path>")
		os.Exit(output.ExitParamError)
		return nil
	}

	req := &model.GetImageRequest{
		WorkspaceID: flagWorkspaceID,
		ImagePath:   flagImagePath,
	}

	img, err := apiClient.GetImage(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, img, !flagPretty)
}

func runImageUpload(cmd *cobra.Command, args []string) error {
	if flagAttachmentEntryID == "" || flagAttachmentField == "" || flagAttachmentFile == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--entry-id, --custom-field, and --file are required",
			"Usage: tapd image upload --entry-id <id> --custom-field <field> --file <path>")
		os.Exit(output.ExitParamError)
		return nil
	}

	data, err := os.ReadFile(flagAttachmentFile)
	if err != nil {
		output.PrintError(os.Stderr, "file_error", err.Error(), "")
		os.Exit(output.ExitParamError)
		return nil
	}

	base64Data := base64.StdEncoding.EncodeToString(data)

	req := &model.UploadImageBase64Request{
		WorkspaceID: flagWorkspaceID,
		Base64Data:  base64Data,
		Type:        "story_custom_field",
		CustomField: flagAttachmentField,
		EntryID:     flagAttachmentEntryID,
	}
	att, err := apiClient.UploadImageBase64(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, att, !flagPretty)
}
