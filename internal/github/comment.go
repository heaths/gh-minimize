package github

import (
	"fmt"
	"slices"
	"strings"
)

var commentFields = []string{
	"id",
	"author",
	"body",
	"isMinimized",
	"minimizedReason",
}

type Comment struct {
	ID              string `json:"id"`
	Author          string `json:"author"`
	Body            string `json:"body"`
	IsMinimized     bool   `json:"isMinimized"`
	MinimizedReason string `json:"minimizedReason"`
}

type graphqlComment struct {
	ID              string        `json:"id"`
	Author          *graphqlActor `json:"author"`
	BodyText        string        `json:"bodyText"`
	IsMinimized     bool          `json:"isMinimized"`
	MinimizedReason string        `json:"minimizedReason"`
}

type graphqlActor struct {
	Login string `json:"login"`
}

func (c graphqlComment) Comment() Comment {
	comment := Comment{
		ID:              c.ID,
		Body:            c.BodyText,
		IsMinimized:     c.IsMinimized,
		MinimizedReason: c.MinimizedReason,
	}
	if c.Author != nil {
		comment.Author = c.Author.Login
	}

	return comment
}

func CommentFields() []string {
	return slices.Clone(commentFields)
}

func ParseCommentFields(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return CommentFields(), nil
	}

	fields := strings.Split(value, ",")
	for i, field := range fields {
		fields[i] = strings.TrimSpace(field)
	}

	if err := ValidateCommentFields(fields); err != nil {
		return nil, err
	}

	return fields, nil
}

func ValidateCommentFields(fields []string) error {
	for _, field := range fields {
		if field == "" {
			return fmt.Errorf("JSON fields cannot be empty")
		}
		if !slices.Contains(commentFields, field) {
			return fmt.Errorf("unknown JSON field %q (available: %s)", field, strings.Join(commentFields, ","))
		}
	}

	return nil
}

func (c Comment) ExportData(fields []string) (map[string]interface{}, error) {
	if err := ValidateCommentFields(fields); err != nil {
		return nil, err
	}

	data := make(map[string]interface{}, len(fields))
	for _, field := range fields {
		switch field {
		case "id":
			data[field] = c.ID
		case "author":
			data[field] = c.Author
		case "body":
			data[field] = c.Body
		case "isMinimized":
			data[field] = c.IsMinimized
		case "minimizedReason":
			data[field] = c.MinimizedReason
		}
	}

	return data, nil
}

func ExportComments(comments []Comment, fields []string) ([]map[string]interface{}, error) {
	if err := ValidateCommentFields(fields); err != nil {
		return nil, err
	}

	data := make([]map[string]interface{}, 0, len(comments))
	for _, comment := range comments {
		exported, err := comment.ExportData(fields)
		if err != nil {
			return nil, err
		}
		data = append(data, exported)
	}

	return data, nil
}
