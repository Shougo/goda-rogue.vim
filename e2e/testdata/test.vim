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

" The lines of the mini-map window, or [] when there is none.
function! s:map_lines() abort
  if has('nvim')
    for winid in nvim_list_wins()
      if nvim_win_get_config(winid).relative !=# ''
        return getbufline(winbufnr(winid), 1, '$')
      endif
    endfor
    return []
  endif
  let popups = popup_list()
  return empty(popups) ? [] : getbufline(winbufnr(popups[0]), 1, '$')
endfunction

" Under the map: the status line and three lines of messages.
function! s:status() abort
  return getline(line('$') - 3)
endfunction

" The messages shown (the empty lines left out).
function! s:messages() abort
  return filter(getline(line('$') - 2, '$'), {_, l -> l !=# ''})
endfunction

function! s:start() abort
  " Without a dungeon, the game asks which.
  let g:goda_rogue#prompt_func = function('s:fake_prompt')
  let s:asked = {}
  call goda_rogue#start(#{seed: 'e2e'})
  call s:check('dungeon prompt', get(s:asked, 'title', ''), 'ダンジョン')
  call s:check('dungeon choice', get(s:asked, 'choices', [''])[0] =~# '^はじまりの洞窟（30F）', 1)
  call s:answer(1)
  unlet g:goda_rogue#prompt_func
  call s:check('filetype', &filetype, 'goda-rogue')
  call s:check('modifiable', &modifiable, 0)

  let width = winwidth(0) - getwininfo(win_getid())[0].textoff
  let tiles = min([width / 2, 66])
  let rows = min([winheight(0) - 5, 36])
  call s:check('lines', line('$'), rows + 5)
  call s:check('top line', getline(1), '1F  Lv1  HP  15/ 15             満腹 100%  0G')
  call s:check('hp bar', s:highlights(1), [[21, 31, 'GodaRogueHP']])
  for lnum in range(2, rows + 1)
    call s:check('tiles of ' . lnum, strchars(getline(lnum)), tiles)
    call s:check('width of ' . lnum, strdisplaywidth(getline(lnum)), tiles * 2)
  endfor
  call s:check('status', s:status() =~# '^攻撃 9  防御 0  ちから 8  ターン 0（この階 0）  歩数 0  時間 0:00:0\d  向き ↓  シード e2e$', 1)
  call s:check('message', s:messages(), ['1F に着いた。'])

  call s:check('cursor', s:cursor_char(), '主')
  call s:check('player highlight',
        \ filter(s:highlights(line('.')), {_, h -> h[0] == col('.')}),
        \ [[col('.'), col('.') + 3, 'GodaRoguePlayer']])
  let groups = s:all_groups()
  for group in ['GodaRogueWall', 'GodaRogueFloor', 'GodaRoguePlayer']
    call s:check('group ' . group, index(groups, group) >= 0, 1)
  endfor
  call s:check('ally', len(filter(copy(groups), {_, g -> g =~# '^GodaRogueAlly[1-4]$'})), 1)

  let dungeons = goda_rogue#dungeons()
  call s:check('dungeons', index(map(copy(dungeons), 'v:val.id'), 'cave') >= 0, 1)
  call s:check('dungeon floors', filter(copy(dungeons), 'v:val.id ==# "cave"')[0].floors, 30)

  let first = getline(1, '$')
  call goda_rogue#start(#{dungeon: 'cave', seed: 'e2e'})
  call s:check('same seed', getline(1, '$'), first)
endfunction

" A stand-in for the selection UI: it keeps the prompt and the function to
" answer it.
function! s:fake_prompt(prompt, answer) abort
  let s:asked = a:prompt
  let s:answer = a:answer
endfunction

function! s:items() abort
  " Walk until something is picked up (another run if the player falls).
  let dirs = ['right', 'down', 'left', 'up', 'downright', 'upleft', 'downleft', 'upright']
  for seed in range(20)
    call goda_rogue#start(#{dungeon: 'cave', seed: 'items' . seed})
    for n in range(100)
      call goda_rogue#do('dash', dirs[n % len(dirs)])
      call goda_rogue#do('pickup')
      if join(s:messages()) =~# 'を拾った。\|冒険は終わった'
        break
      endif
    endfor
    if join(s:messages()) =~# 'を拾った。'
      break
    endif
  endfor
  call s:check('picked up', join(s:messages()) =~# 'を拾った。', 1)

  let g:goda_rogue#prompt_func = function('s:fake_prompt')
  let s:asked = {}
  call goda_rogue#do('inventory')
  call s:check('inventory title', get(s:asked, 'title', ''), '持ち物')
  " An icon, a column marking the equipped items, and the name.
  call s:check('inventory icon', get(s:asked, 'choices', [''])[0] =~# '^\S [ E] \S', 1)
  let name = substitute(s:asked.choices[0], '^\(\S\) [ E] ', '\1 ', '')
  call s:answer(1)
  call s:check('item prompt', s:asked.title, name)
  call s:check('item commands', s:asked.choices[-2:], ['置く', '説明'])
  let turn = s:status()
  call s:answer(0)
  call s:check('cancel', s:status(), turn)
  unlet g:goda_rogue#prompt_func

  if !has('nvim')
    " The popup menu of Vim: its choice is the answer.
    call goda_rogue#do('inventory')
    call s:check('popup', len(popup_list()), 1)
    call popup_close(popup_list()[0], 1)
    call s:check('popup item', len(popup_list()), 1)
    call goda_rogue#do('wait')
    call s:check('popup closed', popup_list(), [])
  endif
endfunction

function! s:actions() abort
  call goda_rogue#start(#{dungeon: 'cave', seed: 42})
  call s:check('number seed', s:status() =~# 'シード 42$', 1)

  call goda_rogue#do('turn')
  call s:check('turn to an enemy', s:status() =~# '  ターン 0（', 1)
  call goda_rogue#do('turn', 'upleft')
  call s:check('turn', s:status() =~# '  ターン 0（.*  向き ↖ ', 1)

  " A move in some direction succeeds: the turn goes on.
  for dir in ['left', 'right', 'up', 'down', 'upleft', 'upright', 'downleft', 'downright']
    call goda_rogue#do('move', dir)
    if s:status() =~# 'ターン 1（'
      break
    endif
  endfor
  call s:check('move', s:status() =~# '  ターン 1（', 1)
  call s:check('cursor after move', s:cursor_char(), '主')
  " Walked tiles show as safe (the ally following the player may stand on
  " the first one).
  for dir in ['left', 'right', 'up', 'down', 'left', 'right', 'up', 'down']
    if index(s:all_groups(), 'GodaRogueFloorSafe') >= 0
      break
    endif
    call goda_rogue#do('move', dir)
  endfor
  call s:check('safe floor', index(s:all_groups(), 'GodaRogueFloorSafe') >= 0, 1)

  call goda_rogue#do('stairs')
  call s:check('no stairs', s:messages()[-1], 'ここに階段はない。')

  call s:check('facing', index(s:all_groups(), 'GodaRogueFacing') >= 0, 1)
  call goda_rogue#do('attack')
  let s:last = s:messages()[-1]
  call s:check('attack', join(s:messages()) =~# '\%(空振りした。\|仲間は攻撃しない。\|のダメージ。\|にかわされた。\)', 1)

  " The log: all the messages, in a buffer.
  let game = win_getid()
  call goda_rogue#do('log')
  call s:check('log buffer', bufname('%'), 'goda-rogue://log')
  call s:check('log', index(getline(1, '$'), 'ここに階段はない。') >= 0, 1)
  call s:check('log last', getline('$'), s:last)
  close
  call win_gotoid(game)

  " The mini-map: a popup / floating window, toggled.
  let status = s:status()
  call goda_rogue#do('map')
  call s:check('map costs no turn', s:status(), status)
  let map = s:map_lines()
  call s:check('map lines', len(map) > 0 && len(map) <= 36, 1)
  call s:check('map player', len(filter(copy(map), {_, l -> l =~# '@'})), 1)
  call s:check('map width', strwidth(get(map, 0, '')) <= 66, 1)
  call goda_rogue#do('wait')
  call s:check('map stays', len(s:map_lines()) > 0, 1)
  call goda_rogue#do('map')
  call s:check('map closed', s:map_lines(), [])

  " Leaving the game buffer closes it, and coming back shows it again.
  call goda_rogue#do('map')
  let game = bufnr('%')
  enew
  call s:check('map closed elsewhere', s:map_lines(), [])
  execute 'buffer' game
  call s:check('map back', len(s:map_lines()) > 0, 1)
  call goda_rogue#do('map')

  " An error is printed, not thrown, and the screen stays.
  let before = getline(1, '$')
  call goda_rogue#do('fly')
  call s:check('unknown action', getline(1, '$'), before)
  call s:check('error message', execute('messages') =~# 'unknown action "fly"', 1)

  call s:items()

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
