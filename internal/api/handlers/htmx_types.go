package handlers

// FileTreeRenderData holds data for rendering the file tree partial.
type FileTreeRenderData struct {
	RootItems   []FileSystemItem `json:"root_items"`
	CurrentPath string           `json:"current_path"`
	ProjectPath string           `json:"project_path"`
}

// FileSystemItem represents a file or directory in the tree.
type FileSystemItem struct {
	Name     string           `json:"name"`
	Path     string           `json:"path"`
	IsDir    bool             `json:"is_dir"`
	Children []FileSystemItem `json:"children,omitempty"`
	Size     int64            `json:"size,omitempty"`
}
