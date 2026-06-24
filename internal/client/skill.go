// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package client

import (
	"bytes"
	"context"
	"errors"
	"io"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Skill is a reusable bundle of instructions and files uploaded to the
// organization. Its id is prefixed with "skill_". Each upload of files creates
// a new SkillVersion; LatestVersion tracks the most recent one. DisplayTitle
// and LatestVersion are empty when unset.
type Skill struct {
	ID            string
	Type          string
	DisplayTitle  string
	LatestVersion string
	Source        string
	CreatedAt     string
	UpdatedAt     string
}

// SkillVersion is an immutable snapshot of a skill's files. Name, Description,
// and Directory are extracted from the uploaded SKILL.md. Version is a Unix
// epoch timestamp string (e.g. "1759178010641129").
type SkillVersion struct {
	ID          string
	Type        string
	SkillID     string
	Version     string
	Name        string
	Description string
	Directory   string
	CreatedAt   string
}

// SkillFile is a single file in a skill upload. Path is the multipart filename
// and must include the top-level directory (e.g. "my-skill/SKILL.md"); the
// upload must contain a SKILL.md at the root of that directory.
type SkillFile struct {
	Path    string
	Content []byte
}

// CreateSkill creates a new skill. When displayTitle is non-empty it is sent as
// a form field; files seed the skill's first version.
func (c *Client) CreateSkill(ctx context.Context, displayTitle string, files []SkillFile) (*Skill, error) {
	params := anthropic.BetaSkillNewParams{Files: skillReaders(files)}
	if displayTitle != "" {
		params.DisplayTitle = anthropic.String(displayTitle)
	}

	b := c.beta()
	res, err := b.Skills.New(ctx, params)
	if err != nil {
		return nil, skillError(err)
	}
	return &Skill{
		ID:            res.ID,
		Type:          res.Type,
		DisplayTitle:  res.DisplayTitle,
		LatestVersion: res.LatestVersion,
		Source:        res.Source,
		CreatedAt:     res.CreatedAt,
		UpdatedAt:     res.UpdatedAt,
	}, nil
}

// GetSkill fetches a single skill by id.
func (c *Client) GetSkill(ctx context.Context, id string) (*Skill, error) {
	b := c.beta()
	res, err := b.Skills.Get(ctx, id, anthropic.BetaSkillGetParams{})
	if err != nil {
		return nil, skillError(err)
	}
	return &Skill{
		ID:            res.ID,
		Type:          res.Type,
		DisplayTitle:  res.DisplayTitle,
		LatestVersion: res.LatestVersion,
		Source:        res.Source,
		CreatedAt:     res.CreatedAt,
		UpdatedAt:     res.UpdatedAt,
	}, nil
}

// DeleteSkill permanently deletes a skill and all of its versions.
func (c *Client) DeleteSkill(ctx context.Context, id string) error {
	b := c.beta()
	_, err := b.Skills.Delete(ctx, id, anthropic.BetaSkillDeleteParams{})
	return skillError(err)
}

// CreateSkillVersion uploads files as a new version of an existing skill.
func (c *Client) CreateSkillVersion(ctx context.Context, skillID string, files []SkillFile) (*SkillVersion, error) {
	b := c.beta()
	res, err := b.Skills.Versions.New(ctx, skillID, anthropic.BetaSkillVersionNewParams{Files: skillReaders(files)})
	if err != nil {
		return nil, skillError(err)
	}
	return skillVersionFromResponse(res.ID, res.Type, res.SkillID, res.Version, res.Name, res.Description, res.Directory, res.CreatedAt), nil
}

// GetSkillVersion fetches a single version of a skill.
func (c *Client) GetSkillVersion(ctx context.Context, skillID, version string) (*SkillVersion, error) {
	b := c.beta()
	res, err := b.Skills.Versions.Get(ctx, version, anthropic.BetaSkillVersionGetParams{SkillID: skillID})
	if err != nil {
		return nil, skillError(err)
	}
	return skillVersionFromResponse(res.ID, res.Type, res.SkillID, res.Version, res.Name, res.Description, res.Directory, res.CreatedAt), nil
}

// DeleteSkillVersion deletes a single version of a skill.
func (c *Client) DeleteSkillVersion(ctx context.Context, skillID, version string) error {
	b := c.beta()
	_, err := b.Skills.Versions.Delete(ctx, version, anthropic.BetaSkillVersionDeleteParams{SkillID: skillID})
	return skillError(err)
}

// beta returns an anthropic-sdk-go client configured with this client's bearer
// token, base URL, and user agent. The Skills endpoints live in the SDK's beta
// surface, which sets the required anthropic-beta header automatically.
func (c *Client) beta() anthropic.BetaService {
	opts := []option.RequestOption{
		option.WithAuthToken(c.token),
		option.WithBaseURL(c.baseURL),
	}
	if c.userAgent != "" {
		opts = append(opts, option.WithHeader("user-agent", c.userAgent))
	}
	if c.httpClient != nil {
		opts = append(opts, option.WithHTTPClient(c.httpClient))
	}
	return anthropic.NewBetaService(opts...)
}

// skillReaders adapts the file list to the SDK's upload type. anthropic.File
// preserves the directory-prefixed Path verbatim as the multipart filename,
// which the API uses to determine the skill's directory and SKILL.md root.
func skillReaders(files []SkillFile) []io.Reader {
	if len(files) == 0 {
		return nil
	}
	readers := make([]io.Reader, 0, len(files))
	for _, f := range files {
		readers = append(readers, anthropic.File(bytes.NewReader(f.Content), f.Path, "application/octet-stream"))
	}
	return readers
}

func skillVersionFromResponse(id, typ, skillID, version, name, description, directory, createdAt string) *SkillVersion {
	return &SkillVersion{
		ID:          id,
		Type:        typ,
		SkillID:     skillID,
		Version:     version,
		Name:        name,
		Description: description,
		Directory:   directory,
		CreatedAt:   createdAt,
	}
}

// skillError translates an anthropic-sdk-go API error into the package's
// *APIError so callers can keep using NotFound and get consistent messages.
func skillError(err error) error {
	if err == nil {
		return nil
	}
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		body := apiErr.RawJSON()
		if body == "" {
			body = apiErr.Error()
		}
		return &APIError{StatusCode: apiErr.StatusCode, Body: body}
	}
	return err
}
