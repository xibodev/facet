#!/usr/bin/env bash
# Facet Video Kit - macOS / Linux Installer
# Usage: ./install.sh
# or one-liner: curl -fsSL https://raw.githubusercontent.com/xibodev/facet/main/install.sh | bash

set -e

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
WHITE='\033[1;37m'
NC='\033[0m'

echo ""
echo -e "${CYAN}======================================================${NC}"
echo -e "${CYAN}      Facet - Autonomous Video Production Kit         ${NC}"
echo -e "${CYAN}======================================================${NC}"
echo ""

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 1. Check Go Compiler
echo -e "${YELLOW}==> Checking Go environment...${NC}"
if ! command -v go &> /dev/null; then
    echo "Error: Go compiler was not found in PATH. Please install Go 1.22+ from https://go.dev and rerun." >&2
    exit 1
fi
echo -e "${GREEN}  Found Go compiler: $(go version)${NC}"

# 2. Build Binaries
echo -e "${YELLOW}==> Compiling Facet binaries (facet, facet-ui)...${NC}"
BIN_DIR="${SCRIPT_DIR}/bin"
mkdir -p "${BIN_DIR}"

FACET_BIN="${BIN_DIR}/facet"
FACET_UI_BIN="${BIN_DIR}/facet-ui"

cd "${SCRIPT_DIR}"
echo "  Building bin/facet..."
go build -o "${FACET_BIN}" ./cmd/facet
chmod +x "${FACET_BIN}"

echo "  Building bin/facet-ui..."
go build -o "${FACET_UI_BIN}" ./cmd/facet-ui
chmod +x "${FACET_UI_BIN}"

echo -e "${GREEN}  Successfully built binaries in ${BIN_DIR}${NC}"

# 3. Setup User Install Directory
USER_BIN_DIR="${HOME}/.facet/bin"
mkdir -p "${USER_BIN_DIR}"
cp -f "${FACET_BIN}" "${USER_BIN_DIR}/facet"
cp -f "${FACET_UI_BIN}" "${USER_BIN_DIR}/facet-ui"
chmod +x "${USER_BIN_DIR}/facet" "${USER_BIN_DIR}/facet-ui"

export PATH="${USER_BIN_DIR}:${PATH}"

# Check shell profile for PATH
PROFILE_HINT=""
if [[ ":$PATH:" != *":${USER_BIN_DIR}:"* ]]; then
    if [ -n "$ZSH_VERSION" ] || [ -f "$HOME/.zshrc" ]; then
        PROFILE_HINT="export PATH=\"\$HOME/.facet/bin:\$PATH\" >> ~/.zshrc"
    else
        PROFILE_HINT="export PATH=\"\$HOME/.facet/bin:\$PATH\" >> ~/.bashrc"
    fi
fi

# 4. Setup Central Bundle
USER_BUNDLE_DIR="${HOME}/.facet/bundle"
mkdir -p "${USER_BUNDLE_DIR}"
for folder in skills pipeline_defs schemas styles; do
    if [ -d "${SCRIPT_DIR}/${folder}" ]; then
        mkdir -p "${USER_BUNDLE_DIR}/${folder}"
        cp -rf "${SCRIPT_DIR}/${folder}/"* "${USER_BUNDLE_DIR}/${folder}/" 2>/dev/null || true
    fi
done
echo -e "${GREEN}  Installed central bundle to ${USER_BUNDLE_DIR}${NC}"

# 5. Register Agent Skills
echo -e "${YELLOW}==> Registering Facet skills for Agent CLIs...${NC}"
SKILLS_SRC="${USER_BUNDLE_DIR}/skills"
if [ -d "${SKILLS_SRC}" ]; then
    # 5a. Claude Code
    CLAUDE_DIR="${HOME}/.claude/skills"
    mkdir -p "${CLAUDE_DIR}"
    rm -rf "${CLAUDE_DIR}/facet"
    ln -sf "${SKILLS_SRC}" "${CLAUDE_DIR}/facet" 2>/dev/null || cp -rf "${SKILLS_SRC}" "${CLAUDE_DIR}/facet"
    echo -e "${GREEN}  Linked skills -> ~/.claude/skills/facet${NC}"

    # 5b. OpenCode
    OPENCODE_DIR="${HOME}/.config/opencode/skills"
    mkdir -p "${OPENCODE_DIR}"
    rm -rf "${OPENCODE_DIR}/facet"
    ln -sf "${SKILLS_SRC}" "${OPENCODE_DIR}/facet" 2>/dev/null || cp -rf "${SKILLS_SRC}" "${OPENCODE_DIR}/facet"
    echo -e "${GREEN}  Linked skills -> ~/.config/opencode/skills/facet${NC}"
fi

# 6. Run Doctor
echo ""
echo -e "${YELLOW}==> Running Facet Doctor...${NC}"
echo ""
"${FACET_BIN}" doctor

echo ""
echo -e "${GREEN}======================================================${NC}"
echo -e "${GREEN}  Facet Installation & Registration Complete!         ${NC}"
echo -e "${GREEN}======================================================${NC}"
echo ""
if [ -n "$PROFILE_HINT" ]; then
    echo -e "${YELLOW}Tip: To add facet to your persistent PATH, run:${NC}"
    echo -e "  echo '${PROFILE_HINT}'"
    echo ""
fi
echo -e "${CYAN}Quick Start:${NC}"
echo -e "${WHITE}  facet ui                  # Launch Facet Studio in browser (:8787)${NC}"
echo -e "${WHITE}  facet init <project-name> # Initialize a new video workspace${NC}"
echo -e "${WHITE}  facet doctor              # Verify system runtimes & tools${NC}"
echo -e "${WHITE}  facet tools list          # List 33 available toolbox tools${NC}"
echo ""
