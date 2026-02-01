# Bashrc - Linux shell configuration

# If not running interactively, don't do anything
[[ $- != *i* ]] && return

# Prompt
PS1='\[\033[01;32m\]\u@\h\[\033[00m\]:\[\033[01;34m\]\w\[\033[00m\]\$ '

# History settings
HISTSIZE=10000
HISTFILESIZE=20000
HISTCONTROL=ignoreboth

# Aliases
alias ls='ls --color=auto'
alias ll='ls -lah'
alias la='ls -A'
alias grep='grep --color=auto'
alias ..='cd ..'
alias ...='cd ../..'

# Git aliases
alias gs='git status'
alias ga='git add'
alias gc='git commit'
alias gp='git push'
alias gl='git log --oneline --graph --decorate'

# Development aliases
alias py='python3'
alias pip='python3 -m pip'

# Enable bash completion
if [ -f /etc/bash_completion ]; then
    . /etc/bash_completion
fi

# Custom additions
export EDITOR=vim
export VISUAL=vim
export PATH="$HOME/.local/bin:$PATH"
