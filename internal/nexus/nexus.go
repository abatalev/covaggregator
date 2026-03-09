package nexus

import (
	"strings"

	"github.com/abatalev/covaggregator/internal/config"
)

func RenderURL(pattern string, sv config.ServiceVersion, artifactID, suffix, repo string) string {
	result := pattern
	groupPath := strings.ReplaceAll(sv.GroupID, ".", "/")
	result = strings.ReplaceAll(result, "{{groupId}}", sv.GroupID)
	result = strings.ReplaceAll(result, "{{group_id}}", sv.GroupID)
	result = strings.ReplaceAll(result, "{{groupPath}}", groupPath)
	result = strings.ReplaceAll(result, "{{artifactId}}", artifactID)
	result = strings.ReplaceAll(result, "{{artifact_id}}", artifactID)
	result = strings.ReplaceAll(result, "{{service}}", sv.Service)
	result = strings.ReplaceAll(result, "{{version}}", sv.Version)
	result = strings.ReplaceAll(result, "{{suffix}}", suffix)
	result = strings.ReplaceAll(result, "{{repository}}", repo)
	return result
}

func SourcesURL(pattern string, sv config.ServiceVersion, artifactID, repo string) string {
	return RenderURL(pattern, sv, artifactID, "-sources", repo)
}

func ClassesURL(pattern string, sv config.ServiceVersion, artifactID, repo string) string {
	return RenderURL(pattern, sv, artifactID, "", repo)
}
