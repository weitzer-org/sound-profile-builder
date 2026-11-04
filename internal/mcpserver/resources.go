package mcpserver

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/weitzer-org/sound-builder/internal/presetsvc"
)

// registerResources registers the read-only resource templates (DESIGN §5.2).
func registerResources(s *mcp.Server, svc *presetsvc.Service) {}

// registerPrompts registers the design_tone prompt (DESIGN §5.3).
func registerPrompts(s *mcp.Server) {}
