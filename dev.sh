#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR=$(realpath "${1:-$PWD}")

if [ ! -d "$PROJECT_DIR" ]; then
  printf 'Directory not found: %s\n' "$PROJECT_DIR" >&2
  exit 1
fi

LAYOUT=$(cat <<EOF
layout cwd="$PROJECT_DIR" {
    default_tab_template {
        pane size=1 borderless=true {
            plugin location="zellij:tab-bar"
        }
        children
        pane size=2 borderless=true {
            plugin location="zellij:status-bar"
        }
    }

    tab name="back" {
        pane command="zsh" close_on_exit=true {
            args "-ic" "cd \"$PROJECT_DIR/\" && nvim; exec zsh"
        }
    }
    tab name="oc" {
        pane command="zsh" close_on_exit=true {
            args "-ic" "cd \"$PROJECT_DIR/\" && ~/dotfiles/scripts/opencode/run.sh; exec zsh"
        }
    }
    tab name=" " {
        pane command="zsh" close_on_exit=true {
            args "-ic" "cd \"$PROJECT_DIR/backend\"; exec zsh"
        }
    }
    tab name="front" {
        pane command="zsh" close_on_exit=true {
            args "-ic" "cd \"$PROJECT_DIR/frontend\" && nvim; exec zsh"
        }
    }
    tab name="oc" {
        pane command="zsh" close_on_exit=true {
            args "-ic" "cd \"$PROJECT_DIR/\" && ~/dotfiles/scripts/opencode/run.sh; exec zsh"
        }
    }
    tab name=" " {
        pane command="zsh" close_on_exit=true {
            args "-ic" "cd \"$PROJECT_DIR/frontend\"; exec zsh"
        }
    }
    // tab name="infra" {
    //     pane command="zsh" close_on_exit=true {
    //         args "-ic" "cd \"$PROJECT_DIR/infra\" && nvim; exec zsh"
    //     }
    // }
    // tab name="oc" {
    //     pane command="zsh" close_on_exit=true {
    //         args "-ic" "cd \"$PROJECT_DIR/\" && ~/dotfiles/scripts/opencode/run.sh; exec zsh"
    //     }
    // }
    // tab name=" " {
    //     pane command="zsh" close_on_exit=true {
    //         args "-ic" "cd \"$PROJECT_DIR/infra\"; exec zsh"
    //     }
    // }
    tab name="back-dev" {
        pane command="zsh" close_on_exit=true {
            args "-ic" "cd \"$PROJECT_DIR/backend\" && docker compose up -d postgres seaweedfs && ./dev.sh; exec zsh"
        }
    }
    tab name="front-dev" {
        pane command="zsh" close_on_exit=true {
            args "-ic" "cd \"$PROJECT_DIR/frontend\" && npm run dev; exec zsh"
        }
    }
}
EOF
)

exec zellij --layout-string "$LAYOUT"
