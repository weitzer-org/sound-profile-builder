import json

file_path = "/usr/local/google/home/benweitzer/Documents/sound-profile-builder/internal/agents/coros_map.json"

with open(file_path, 'r', encoding='utf-8') as f:
    content = f.read()

# Replace smart quote with standard straight quote
fixed_content = content.replace("’", "'").replace("\\u2019", "'")

with open(file_path, 'w', encoding='utf-8') as f:
    f.write(fixed_content)

print("Smart quotes replaced successfully.")
