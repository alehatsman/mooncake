" Vimrc - Vim configuration

" Basic settings
set number
set relativenumber
set expandtab
set tabstop=2
set shiftwidth=2
set smartindent
set nowrap
set noswapfile
set nobackup
set hlsearch
set incsearch
set scrolloff=8
set updatetime=50
set signcolumn=yes
set colorcolumn=80

" Enable syntax highlighting
syntax on

" Enable file type detection
filetype plugin indent on

" Leader key
let mapleader = " "

" Key mappings
nnoremap <leader>w :w<CR>
nnoremap <leader>q :q<CR>
nnoremap <C-d> <C-d>zz
nnoremap <C-u> <C-u>zz
nnoremap n nzzzv
nnoremap N Nzzzv

" Split navigation
nnoremap <C-h> <C-w>h
nnoremap <C-j> <C-w>j
nnoremap <C-k> <C-w>k
nnoremap <C-l> <C-w>l

" Buffer navigation
nnoremap <leader>bn :bnext<CR>
nnoremap <leader>bp :bprev<CR>
nnoremap <leader>bd :bdelete<CR>

" Clear highlighting
nnoremap <leader>h :noh<CR>

" Auto commands
autocmd BufWritePre * :%s/\s\+$//e  " Remove trailing whitespace

" Status line
set laststatus=2
set statusline=%F%m%r%h%w\ [%l/%L,%c]\ [%p%%]
