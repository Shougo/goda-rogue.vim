" The end-to-end scenario: the game in a real Vim or Neovim. Failures go to
" $GODA_ROGUE_RESULT; it says "ok" at the end.

set nocompatible
set encoding=utf-8
set nomore
syntax on
let s:errors = []
let s:result = $GODA_ROGUE_RESULT

execute 'set runtimepath^=' . fnameescape($GODA_VIM_DIR)
execute 'set runtimepath^=' . fnameescape($GODA_ROGUE_DIR)
let g:goda_rogue#binary = $GODA_ROGUE_BINARY

function! s:check(name, got, want) abort
  if a:got !=# a:want
    call add(s:errors, printf('%s: got %s, want %s', a:name, string(a:got), string(a:want)))
  endif
endfunction

" The highlight groups of the line {lnum}: [[col, end, group]].
function! s:highlights(lnum) abort
  if has('nvim')
    let ns = nvim_get_namespaces()['goda-rogue']
    return map(nvim_buf_get_extmarks(0, ns, [a:lnum - 1, 0], [a:lnum - 1, -1], #{details: v:true}),
          \ {_, m -> [m[2] + 1, m[3].end_col + 1, m[3].hl_group]})
  endif
  return map(prop_list(a:lnum), {_, p -> [p.col, p.col + p.length, p.type]})
endfunction

function! s:all_groups() abort
  let groups = {}
  for lnum in range(1, line('$'))
    for [_, _, group] in s:highlights(lnum)
      let groups[group] = 1
    endfor
  endfor
  return sort(keys(groups))
endfunction

" The character under the cursor.
function! s:cursor_char() abort
  return strcharpart(getline('.')[col('.') - 1 :], 0, 1)
endfunction

function! s:status() abort
  return getline(line('$') - 1)
endfunction

function! s:start() abort
  call goda_rogue#start(#{seed: 'e2e'})
  call s:check('filetype', &filetype, 'goda-rogue')
  call s:check('modifiable', &modifiable, 0)

  let width = winwidth(0) - getwininfo(win_getid())[0].textoff
  let tiles = min([width / 2, 63])
  let rows = min([winheight(0) - 2, 33])
  call s:check('lines', line('$'), rows + 2)
  for lnum in range(1, rows)
    call s:check('tiles of ' . lnum, strchars(getline(lnum)), tiles)
    call s:check('width of ' . lnum, strdisplaywidth(getline(lnum)), tiles * 2)
  endfor
  call s:check('status', s:status() =~# '^1F  ターン 0  向き 下  シード e2e$', 1)
  call s:check('message', getline('$'), '1F に着いた。')

  call s:check('cursor', s:cursor_char(), '主')
  call s:check('player highlight',
        \ filter(s:highlights(line('.')), {_, h -> h[0] == col('.')}),
        \ [[col('.'), col('.') + 3, 'GodaRoguePlayer']])
  let groups = s:all_groups()
  for group in ['GodaRogueWall', 'GodaRogueFloor', 'GodaRoguePlayer']
    call s:check('group ' . group, index(groups, group) >= 0, 1)
  endfor
  call s:check('ally', len(filter(copy(groups), {_, g -> g =~# '^GodaRogueAlly[1-4]$'})), 1)

  let first = getline(1, '$')
  call goda_rogue#start(#{seed: 'e2e'})
  call s:check('same seed', getline(1, '$'), first)
endfunction

function! s:actions() abort
  call goda_rogue#start(#{seed: 42})
  call s:check('number seed', s:status() =~# 'シード 42$', 1)

  call goda_rogue#do('turn', 'upleft')
  call s:check('turn', s:status() =~# '^1F  ターン 0  向き 左上 ', 1)

  " A move in some direction succeeds: the turn goes on.
  for dir in ['left', 'right', 'up', 'down', 'upleft', 'upright', 'downleft', 'downright']
    call goda_rogue#do('move', dir)
    if s:status() =~# 'ターン 1 '
      break
    endif
  endfor
  call s:check('move', s:status() =~# '^1F  ターン 1 ', 1)
  call s:check('cursor after move', s:cursor_char(), '主')

  call goda_rogue#do('stairs')
  call s:check('no stairs', getline('$'), 'ここに階段はない。')

  " An error is printed, not thrown, and the screen stays.
  let before = getline(1, '$')
  call goda_rogue#do('fly')
  call s:check('unknown action', getline(1, '$'), before)
  call s:check('error message', execute('messages') =~# 'unknown action "fly"', 1)

  " Elsewhere it does nothing.
  enew
  call goda_rogue#do('move', 'left')
  call s:check('other buffer', getline(1, '$'), [''])
endfunction

try
  call s:start()
  call s:actions()
catch
  call add(s:errors, v:throwpoint . ': ' . v:exception)
endtry
call writefile(s:errors + (empty(s:errors) ? ['ok'] : []) + goda#log(), s:result)
call goda#stop()
qall!
