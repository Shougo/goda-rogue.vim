" goda-rogue.vim - a mystery-dungeon roguelike for Vim/Neovim on goda.vim.
"
" The Go side (the binary built from this repository) owns the whole game;
" this file sends it the actions and shows the screens it returns: lines,
" highlights, and the cursor on the player.

" The version of this Vim script that the binary goes with (ScriptVersion of
" its Go side): goda.vim refuses a binary of another one, and builds it again.
let s:version = #{version: 1, module: 'github.com/Shougo/goda-rogue.vim'}

" The repository: the Go module of the binary.
let s:root = expand('<sfile>:p:h:h', 1)

let s:ns = has('nvim') ? nvim_create_namespace('goda-rogue') : 0

" Highlights over the others (the background of the tile the player faces).
let s:over = #{GodaRogueFacing: 1}

" ---------------------------------------------------------------------------
" Setup

" The binary is machine specific: it is built into a cache directory.
function! s:cache_dir() abort
  if has('nvim')
    let base = stdpath('cache')
  elseif !empty($XDG_CACHE_HOME)
    let base = $XDG_CACHE_HOME . '/vim'
  elseif has('win32') && !empty($LOCALAPPDATA)
    let base = $LOCALAPPDATA . '/vim'
  else
    let base = expand('~/.cache/vim', 1)
  endif
  return base . '/goda-rogue'
endfunction

function! s:default_bin() abort
  if exists('g:goda_rogue#binary')
    " (Not 'wildignore': '*.exe' in it on Windows made it "".)
    return expand(g:goda_rogue#binary, 1)
  endif
  return s:cache_dir() . '/goda-rogue' . (has('win32') ? '.exe' : '')
endfunction

" The options of goda#register(): the version, and how to build the binary
" when there is none or it is of another version. Not for the binary of
" g:goda_rogue#binary: the user builds it.
function! s:register_opts() abort
  if exists('g:goda_rogue#binary')
    return s:version
  endif
  return extend(#{build: function('s:build_binary'), bin: s:default_bin()}, s:version)
endfunction

function! s:build_binary() abort
  call goda#build(#{name: 'rogue', dir: s:root, bin: s:default_bin()})
endfunction

" Build the binary again (after an update of this plugin, say). The running
" one is stopped, and the new one starts with the next request.
function! goda_rogue#build() abort
  call s:build_binary()
  call goda#register('rogue', function('s:start'), s:register_opts())
endfunction

function! s:start() abort
  let bin = s:default_bin()
  if !executable(bin)
    throw 'goda-rogue: ' . bin . ' does not exist yet; build it with: call goda_rogue#build()'
  endif
  let plugins = goda#start(bin)
  if index(plugins, 'rogue') < 0
    if !empty(plugins)
      call goda#stop(plugins[0])
    endif
    throw 'goda-rogue: ' . bin . ' does not contain the "rogue" plugin'
  endif
endfunction

call goda#register('rogue', function('s:start'), s:register_opts())

" ---------------------------------------------------------------------------
" Highlights: the groups of the Go side. Ranks are colors, and an ally is
" its rank's color, underlined. The default colors depend on 'background'.

" [cterm, gui] colors for a dark and a light background.
let s:colors = #{
      \ dark: #{
      \   wall: ['73', '#5fafaf'], floor: ['240', '#585858'],
      \   safe: ['180', '#d7af87'], stairs: ['220', '#ffd700'],
      \   facing: ['238', '#444444'],
      \   ranks: [['77', '#5fd75f'], ['69', '#5f87ff'],
      \           ['170', '#d75fd7'], ['203', '#ff5f5f']],
      \ },
      \ light: #{
      \   wall: ['30', '#008787'], floor: ['250', '#bcbcbc'],
      \   safe: ['130', '#af5f00'], stairs: ['136', '#af8700'],
      \   facing: ['253', '#dadada'],
      \   ranks: [['28', '#008700'], ['25', '#005faf'],
      \           ['90', '#870087'], ['160', '#d70000']],
      \ },
      \ }

function! s:highlights() abort
  let c = s:colors[&background ==# 'light' ? 'light' : 'dark']
  for [group, color, attr] in [
        \ ['GodaRogueWall', c.wall, ''],
        \ ['GodaRogueFloor', c.floor, ''],
        \ ['GodaRogueFloorSafe', c.safe, ''],
        \ ['GodaRogueStairs', c.stairs, 'bold'],
        \ ]
    call s:highlight(group, color, attr)
  endfor
  " The tile the player faces: a background over the highlight of the tile.
  call s:highlight('GodaRogueFacing', c.facing, '', 'bg')
  highlight default GodaRoguePlayer term=bold cterm=bold gui=bold
  for i in range(len(c.ranks))
    call s:highlight('GodaRogueRank' . (i + 1), c.ranks[i], '')
    call s:highlight('GodaRogueAlly' . (i + 1), c.ranks[i], 'underline')
  endfor
endfunction

" Set the color of {group} by {group}Default: it follows 'background', and
" {group} itself is left to the user. {kind} is 'fg' or 'bg'.
function! s:highlight(group, color, attr, kind = 'fg') abort
  let attr = empty(a:attr) ? ''
        \ : printf(' term=%s cterm=%s gui=%s', a:attr, a:attr, a:attr)
  execute 'highlight clear' a:group . 'Default'
  execute printf('highlight %sDefault cterm%s=%s gui%s=%s%s',
        \ a:group, a:kind, a:color[0], a:kind, a:color[1], attr)
  execute printf('highlight default link %s %sDefault', a:group, a:group)
endfunction

call s:highlights()

augroup goda-rogue
  autocmd!
  autocmd ColorScheme * call s:highlights()
  autocmd OptionSet background call s:highlights()
  autocmd VimResized,WinResized * call s:redraw_all()
augroup END

" ---------------------------------------------------------------------------
" API

" Start a new game in the current window. {options}:
"   seed	The same seed makes the same floors (a string or a number).
"		Default: a random one.
function! goda_rogue#start(options = {}) abort
  let options = copy(a:options)
  if type(get(options, 'seed', '')) == v:t_number
    let options.seed = string(options.seed)
  endif
  let bufnr = bufadd('goda-rogue://game')
  call setbufvar(bufnr, '&buftype', 'nofile')
  call setbufvar(bufnr, '&bufhidden', 'hide')
  call setbufvar(bufnr, '&swapfile', 0)
  call setbufvar(bufnr, '&buflisted', 0)
  call bufload(bufnr)
  execute 'buffer' bufnr
  setlocal nowrap nolist nonumber norelativenumber nospell
  setlocal nocursorline nocursorcolumn signcolumn=no foldcolumn=0 colorcolumn=
  setlocal nomodifiable
  setlocal filetype=goda-rogue
  try
    let screen = goda#request('rogue', 'start', options, s:size())
  catch
    call s:print_error(v:exception)
    return
  endtry
  call s:draw(screen)
endfunction

" Run the action {action} of the game, with its arguments: >
"	call goda_rogue#do('move', 'left')
" Actions: move, dash and turn (with a direction: left, right, up, down,
" upleft, upright, downleft, downright), attack, wait, stairs, redraw.
function! goda_rogue#do(action, ...) abort
  if &filetype !=# 'goda-rogue'
    return
  endif
  try
    let screen = goda#request('rogue', 'do', a:action, a:000, s:size())
  catch
    call s:print_error(v:exception)
    return
  endtry
  call s:draw(screen)
endfunction

" ---------------------------------------------------------------------------
" Drawing

" The size of the text area of the current window.
function! s:size() abort
  let info = getwininfo(win_getid())[0]
  return #{width: winwidth(0) - info.textoff, height: winheight(0)}
endfunction

" Show {screen} of the Go side in the current buffer.
function! s:draw(screen) abort
  let lines = a:screen.lines
  setlocal modifiable
  call setline(1, lines)
  if line('$') > len(lines)
    silent call deletebufline('%', len(lines) + 1, '$')
  endif
  setlocal nomodifiable

  if has('nvim')
    call nvim_buf_clear_namespace(0, s:ns, 0, -1)
    for [group, spans] in items(a:screen.highlights)
      for s in spans
        call nvim_buf_set_extmark(0, s:ns, s.line - 1, s.col - 1,
              \ #{end_col: s.end - 1, hl_group: group,
              \   priority: has_key(s:over, group) ? 4200 : 4096})
      endfor
    endfor
  else
    call prop_clear(1, line('$'))
    for [group, spans] in items(a:screen.highlights)
      if empty(prop_type_get(group))
        call prop_type_add(group, #{highlight: group, combine: v:true,
              \ priority: has_key(s:over, group) ? 10 : 0})
      endif
      call prop_add_list(#{type: group},
            \ map(copy(spans), {_, s -> [s.line, s.col, s.line, s.end]}))
    endfor
  endif

  call cursor(a:screen.cursor[0], a:screen.cursor[1])
endfunction

" Draw the game again in every window that shows it, for its new size.
function! s:redraw_all() abort
  let bufnr = bufnr('goda-rogue://game')
  if bufnr < 0 || !goda#is_running('rogue')
    return
  endif
  for winid in win_findbuf(bufnr)
    call win_execute(winid, 'call goda_rogue#do("redraw")')
  endfor
endfunction

function! s:print_error(msg) abort
  echohl ErrorMsg
  for line in split(a:msg, "\n")
    echomsg line =~# '^goda' ? line : '[goda-rogue] ' . line
  endfor
  echohl None
endfunction
