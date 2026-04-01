import sys

path = 'internal/api/server.go'

try:
    with open(path, 'r') as f:
        content = f.read()
except FileNotFoundError:
    print(f"Error: {path} not found")
    sys.exit(1)

# Add memoryStore to Server struct
if 'memoryStore *storage.MemoryStore' not in content:
    target = 'type Server struct {\n\tmux         *http.ServeMux\n\tstore       *storage.PresetStore'
    replacement = 'type Server struct {\n\tmux         *http.ServeMux\n\tstore       *storage.PresetStore\n\tmemoryStore *storage.MemoryStore'
    content = content.replace(target, replacement)

# Update NewServer
if 'memoryStore *storage.MemoryStore' not in content:
    target = 'func NewServer(store *storage.PresetStore, client storage.Client'
    replacement = 'func NewServer(store *storage.PresetStore, memoryStore *storage.MemoryStore, client storage.Client'
    content = content.replace(target, replacement)

# Update NewServer initialization
if 'memoryStore: memoryStore,' not in content:
    target = 's := &Server{\n\t\tmux:       http.NewServeMux(),\n\t\tstore:     store,'
    replacement = 's := &Server{\n\t\tmux:         http.NewServeMux(),\n\t\tstore:       store,\n\t\tmemoryStore: memoryStore,'
    content = content.replace(target, replacement)

# Add routes
if '/api/memories' not in content:
    target = 's.mux.Handle("/api/preset/delete_draft", s.authMiddleware(s.handleDeleteDraftPreset()))\n}'
    replacement = 's.mux.Handle("/api/preset/delete_draft", s.authMiddleware(s.handleDeleteDraftPreset()))\n\n\t// Memory Rules Routes\n\ts.mux.Handle("/api/memories", s.authMiddleware(s.handleGetMemories()))\n\ts.mux.Handle("/api/memory/delete", s.authMiddleware(s.handleDeleteMemory()))\n\ts.mux.Handle("/api/presets_list_view", s.authMiddleware(s.handlePresetsListView()))\n}'
    content = content.replace(target, replacement)

with open(path, 'w') as f:
    f.write(content)

print("Successfully patched server.go")
