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
" The replays go to a temporary directory.
let g:goda_rogue#data_dir = tempname()

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

" The lines of the window beside the game whose title matches {pattern}, or
" [] when there is none.
function! s:float_lines(pattern) abort
  return get(s:floats(a:pattern), 0, #{lines: []}).lines
endfunction

" The windows beside the game whose titles match {pattern}: their titles,
" lines, and whether they cover whole tiles of the map (from a column where
" a tile starts, an even number of cells wide, the border included).
function! s:floats(pattern) abort
  let floats = []
  let info = getwininfo(win_getid())[0]
  let shift = info.textoff - winsaveview().leftcol
  if has('nvim')
    for winid in nvim_list_wins()
      let config = nvim_win_get_config(winid)
      if config.relative !=# ''
        let title = join(map(copy(get(config, 'title', [])), 'v:val[0]'), '')
        let tiles = (float2nr(config.col) - shift) % 2 == 0 && (config.width + 2) % 2 == 0
        call add(floats, #{title: title, lines: getbufline(winbufnr(winid), 1, '$'),
              \ tiles: tiles})
      endif
    endfor
  else
    for winid in popup_list()
      let pos = popup_getpos(winid)
      let tiles = (pos.col - info.wincol - shift) % 2 == 0 && pos.width % 2 == 0
      call add(floats, #{title: get(popup_getoptions(winid), 'title', ''),
            \ lines: getbufline(winbufnr(winid), 1, '$'), tiles: tiles})
    endfor
  endif
  return filter(floats, {_, f -> f.title =~# a:pattern})
endfunction

" The lines of the mini-map window, or [] when there is none.
function! s:map_lines() abort
  return s:float_lines('^ map $')
endfunction

" The lines of the window of the items, or [] when there is none.
function! s:item_lines() abort
  return s:float_lines('^ 持ち物')
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
  call s:check('dungeon choice', get(s:asked, 'choices', [''])[0], 'はじまりの洞窟（30F）')
  call s:answer(1)
  " Then what to do with it: its description, and back.
  call s:check('dungeon menu', get(s:asked, 'title', ''), 'はじまりの洞窟（30F）')
  call s:check('dungeon menu choices', get(s:asked, 'choices', []),
        \ ['冒険に出る', '説明を見る', 'Bot の冒険を見る', 'やめる'])
  call s:answer(2)
  call s:check('dungeon description', get(s:asked, 'title', '') =~# '^はじまりの洞窟（30F）： .', 1)
  call s:answer(1)
  call s:check('dungeon menu again', get(s:asked, 'choices', [''])[0], '冒険に出る')
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
  call s:check('status', s:status() =~# '^攻撃 9  防御 0  ちから 8/8  持ち物 0/20  ターン 0（この階 0）  歩数 0  時間 0:00:0\d  向き ↓  シード e2e$', 1)
  call s:check('message', s:messages(), ['1F に着いた。'])

  call s:check('cursor', s:cursor_char(), '主')
  " No horizontal scroll for the user's 'sidescrolloff': the long status
  " line would be cut.
  set sidescrolloff=999
  call goda_rogue#do('redraw')
  call s:check('no side scroll', [&l:sidescrolloff, winsaveview().leftcol], [0, 0])
  set sidescrolloff=0
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
      " (Over: no more actions, which would ask what comes next.)
      if search('^  結果 ', 'nw') > 0
        break
      endif
      call goda_rogue#do('pickup')
      " (Not money: it goes to the purse.)
      if s:status() =~# '持ち物 [1-9]' || search('^  結果 ', 'nw') > 0
        break
      endif
    endfor
    if s:status() =~# '持ち物 [1-9]' && search('^  結果 ', 'nw') == 0
      break
    endif
  endfor
  call s:check('picked up', s:status() =~# '持ち物 [1-9]' && search('^  結果 ', 'nw') == 0, 1)

  let g:goda_rogue#prompt_func = function('s:fake_prompt')
  let s:asked = {}
  call goda_rogue#do('inventory')
  call s:check('inventory title', get(s:asked, 'title', ''), '持ち物')
  " An icon, a column marking the equipped items, and the name.
  call s:check('inventory icon', get(s:asked, 'choices', [''])[0] =~# '^\S [ E] \S', 1)
  let name = substitute(s:asked.choices[0], '^\(\S\) [ E] ', '\1 ', '')
  call s:answer(1)
  call s:check('item prompt', s:asked.title, name)
  " (An item not identified has 名前をつける before 説明.)
  call s:check('item commands', [index(s:asked.choices, '置く') >= 0, s:asked.choices[-1]], [1, '説明'])
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
  call goda_rogue#start(#{dungeon: 'cave', seed: 42, debug: v:true})
  call s:check('number seed', s:status() =~# 'シード 42$', 1)
  " The checks below must not depend on the floors: the player cannot
  " fall.
  call goda_rogue#do('debug', 'invincible')

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

  " The items carried: a window too, toggled, beside the mini-map.
  let status = s:status()
  call goda_rogue#do('items')
  call s:check('items cost no turn', s:status(), status)
  call s:check('items title', get(s:floats('^ 持ち物'), 0, #{title: ''}).title =~# '^ 持ち物 \d\+/20 $', 1)
  call s:check('items lines', len(s:item_lines()) > 0, 1)
  " Over whole tiles: a border over half of one is cleared by some
  " terminals. (A fold column of one cell shifts the tiles.)
  setlocal foldcolumn=1
  call goda_rogue#do('redraw')
  call s:check('items on tiles', get(s:floats('^ 持ち物'), 0, #{tiles: 0}).tiles, 1)
  call goda_rogue#do('map')
  call s:check('map on tiles', get(s:floats('^ map $'), 0, #{tiles: 0}).tiles, 1)
  if has('nvim')
    " Not see-through, whatever 'winblend' the user gives them.
    let floats = filter(nvim_list_wins(), {_, w -> nvim_win_get_config(w).relative !=# ''})
    for winid in floats
      call setwinvar(winid, '&winblend', 30)
    endfor
    call goda_rogue#do('redraw')
    call s:check('not see-through', map(floats, {_, w -> getwinvar(w, '&winblend')}), [0, 0])
  endif
  setlocal foldcolumn=0
  call goda_rogue#do('redraw')
  call s:check('items and map', [len(s:item_lines()) > 0, len(s:map_lines()) > 0], [1, 1])
  call goda_rogue#do('map')
  let game = bufnr('%')
  enew
  call s:check('items closed elsewhere', s:item_lines(), [])
  execute 'buffer' game
  call s:check('items back', len(s:item_lines()) > 0, 1)
  call goda_rogue#do('items')
  call s:check('items closed', s:item_lines(), [])

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

" The replay file: suspend, resume, save and watch.
function! s:replays() abort
  call goda_rogue#start(#{dungeon: 'cave', seed: 'replay'})
  for dir in ['right', 'left', 'down', 'up']
    call goda_rogue#do('move', dir)
  endfor
  call goda_rogue#do('wait')
  let status = s:timeless(s:status())
  let file = goda_rogue#last_replay()
  call s:check('replay file', get(readfile(file), 0, ''), '# goda-rogue replay')

  call goda_rogue#do('suspend')
  call s:check('suspended', getline(2), '  冒険を中断した。')
  call goda_rogue#do('wait')
  call s:check('no game after suspending', execute('messages') =~# 'no game', 1)

  " Starting again offers to resume it.
  let g:goda_rogue#prompt_func = function('s:fake_prompt')
  let s:asked = {}
  call goda_rogue#start()
  call s:check('resume prompt', get(s:asked, 'title', '') =~# '^中断した冒険があります（はじまりの洞窟 1F', 1)
  call s:check('resume choices', get(s:asked, 'choices', []), ['再開する', '捨てて新しく始める'])
  call s:answer(1)
  unlet g:goda_rogue#prompt_func
  call s:check('resumed', s:messages()[-1], '冒険を再開した。')
  call s:check('resumed status', s:timeless(s:status()), status)

  let saved = goda_rogue#save_replay('e2e')
  call s:check('saved', fnamemodify(saved, ':t'), 'e2e.replay')
  call s:check('saved file', readfile(saved), readfile(file))

  " Watched: it plays by itself; paused and stepped to its end here.
  call goda_rogue#replay(saved)
  call goda_rogue#replay_pause()
  call s:check('watch', getline('$') =~# '^リプレイ  0/\d\+$', 1)
  call goda_rogue#replay_step(-1)
  call s:check('watched', getline('$') =~# '^リプレイ  \(\d\+\)/\1$', 1)
  call s:check('watched status', s:timeless(getline(line('$') - 4)), status)
  call goda_rogue#do('wait')
  call s:check('no action while watching', execute('messages') =~# 'a replay or a bot is being watched', 1)

  " A bot plays, watched the same way; the suspended run stays.
  call goda_rogue#watch_bot(#{dungeon: 'cave', seed: 'bot'})
  call goda_rogue#replay_pause()
  call s:check('bot', getline('$'), 'Bot（careful）  0 手')
  call s:check('bot floor', getline(1) =~# '^1F ', 1)
  call goda_rogue#replay_step(3)
  call s:check('bot stepped', getline('$'), 'Bot（careful）  3 手')
  call goda_rogue#replay_step(-1)
  call s:check('bot run over', search('^  結果 ', 'nw') > 0, 1)
  call s:check('bot left the replay', readfile(goda_rogue#last_replay()), readfile(saved))
  " The run of the bot, compared with what a bot would have done.
  call goda_rogue#compare_bot(goda_rogue#bot_replay())
  call s:check('compare buffer', bufname('%'), 'goda-rogue://compare')
  call s:check('compare title', getline(1) =~# '^リプレイと Bot（careful）の比較  はじまりの洞窟  シード bot$', 1)
  call s:check('compare counts', getline(2) =~# '^判断 \d\+ 回のうち、', 1)
  call goda_rogue#compare_bot(goda_rogue#bot_replay(), 'reckless')
  call s:check('compare title of a style', getline(1) =~# 'Bot（reckless）', 1)

  " Its replay can be saved under a name, and the run of a bot before is
  " kept beside the next one.
  let botsaved = goda_rogue#save_replay('e2e-bot', goda_rogue#bot_replay())
  call s:check('bot replay saved', readfile(botsaved), readfile(goda_rogue#bot_replay()))
  let first = readfile(goda_rogue#bot_replay())
  let g:goda_rogue#prompt_func = function('s:fake_prompt')
  let s:asked = {}
  call goda_rogue#start()
  if get(s:asked, 'title', '') !=# 'ダンジョン'
    " (The run left behind: not resumed.)
    call s:answer(2)
  endif
  call s:answer(1)
  call s:answer(3)
  call s:check('bot menu', get(s:asked, 'title', ''), 'Bot')
  call s:check('bot menu choices', len(get(s:asked, 'choices', [])), 2)
  call s:answer(2)
  unlet g:goda_rogue#prompt_func
  call goda_rogue#replay_pause()
  call s:check('bot of the menu', getline('$'), 'Bot（reckless）  0 手')
  let kept = glob(g:goda_rogue#data_dir . '/replays/bot/*-cave*.replay', 1, 1)
  call s:check('kept bot replay', len(filter(kept, {_, f -> readfile(f) == first})), 1)
  call goda_rogue#watch_bot(#{dungeon: 'cave', seed: 'bot'})
  call goda_rogue#replay_pause()
  call goda_rogue#replay_step(-1)
  " Its own replay is kept, and can be watched.
  call goda_rogue#replay(goda_rogue#bot_replay())
  call goda_rogue#replay_pause()
  call goda_rogue#replay_step(-1)
  call s:check('bot replay watched', getline('$') =~# '^リプレイ  \(\d\+\)/\1$', 1)
  call s:check('bot replay over', search('^  結果 ', 'nw') > 0, 1)

  " A new run keeps the replay of the one before, not over here.
  let g:goda_rogue#prompt_func = function('s:fake_prompt')
  call goda_rogue#start(#{dungeon: 'cave', seed: 'replay2'})
  call s:answer(2)
  unlet g:goda_rogue#prompt_func
  let auto = glob(g:goda_rogue#data_dir . '/replays/auto/*-cave*.replay', 1, 1)
  call s:check('kept replay', len(filter(auto, {_, f -> readfile(f) == readfile(saved)})), 1)
endfunction

" The debug mode: off by default, on with g:goda_rogue#debug.
function! s:debug() abort
  " (No suspended run to offer to resume.)
  call delete(goda_rogue#last_replay())
  call goda_rogue#start(#{dungeon: 'cave', seed: 'debug'})
  call goda_rogue#do('debug', 'gold')
  call s:check('debug disabled', execute('messages') =~# 'debug: not enabled', 1)

  let g:goda_rogue#debug = 1
  call goda_rogue#start(#{dungeon: 'cave', seed: 'debug'})
  call goda_rogue#do('debug', 'floor', '4')
  call s:check('debug floor', getline(1) =~# '^4F ', 1)
  call s:check('debug status', s:status() =~# '  デバッグ$', 1)
  let g:goda_rogue#prompt_func = function('s:fake_prompt')
  let s:asked = {}
  call goda_rogue#do('debug')
  call s:check('debug menu', get(s:asked, 'title', ''), 'デバッグ')
  call s:answer(index(s:asked.choices, '1000G 得る') + 1)
  call s:check('debug gold', getline(1) =~# '1000G$', 1)

  " Several items put in a pot at once: the prompt takes a list.
  call goda_rogue#do('debug', 'item', 'hozon')
  call goda_rogue#do('debug', 'item', 'yakusou')
  call goda_rogue#do('debug', 'item', 'otogiri')
  " (Unidentified: known by their icons.)
  call goda_rogue#do('inventory')
  call s:answer(match(s:asked.choices, '^壺') + 1)
  call s:answer(index(s:asked.choices, '入れる') + 1)
  call s:check('put in many', get(s:asked, 'multi', v:false), v:true)
  let grasses = filter(range(1, len(s:asked.choices)), {_, i -> s:asked.choices[i - 1] =~# '^草'})
  call s:check('grasses', len(grasses), 2)
  call s:answer(grasses)
  call goda_rogue#do('inventory')
  call s:check('put in', filter(copy(s:asked.choices), {_, c -> c =~# '^草\|^壺'}),
        \ filter(copy(s:asked.choices), {_, c -> c =~# '^壺.*【草草】$'}))
  call goda_rogue#do('cancel')

  " Naming a kind not identified: a text prompt, then the name shows.
  call goda_rogue#do('debug', 'item', 'konran')
  call goda_rogue#do('inventory')
  " (The newest item, the last of its kind: a grass not yet in a pot.)
  call s:answer(match(s:asked.choices, '^草') + 1)
  call s:answer(index(s:asked.choices, '名前をつける') + 1)
  call s:check('name prompt', get(s:asked, 'input', v:false), v:true)
  call s:answer('混乱?')
  call goda_rogue#do('inventory')
  call s:check('named', len(filter(copy(s:asked.choices), {_, c -> c =~# '「混乱?」'})), 1)
  call goda_rogue#do('cancel')
  unlet g:goda_rogue#prompt_func
  unlet g:goda_rogue#debug
endfunction

" The records of the finished runs.
function! s:records() abort
  " A run that ends: waiting, until a monster, hunger or the wind ends it.
  call delete(goda_rogue#last_replay())
  call goda_rogue#start(#{dungeon: 'cave', seed: 'records'})
  for _ in range(3000)
    if search('冒険の結果', 'nw') > 0
      break
    endif
    call goda_rogue#do('wait')
  endfor
  call s:check('a run over', search('冒険の結果', 'nw') > 0, 1)

  " Over, a key asks what comes next: the same dungeon again.
  let g:goda_rogue#prompt_func = function('s:fake_prompt')
  let s:asked = {}
  call goda_rogue#do('wait')
  call s:check('over prompt', get(s:asked, 'title', '') =~# '^冒険は終わった（', 1)
  call s:check('over choices', get(s:asked, 'choices', []),
        \ ['同じダンジョンに再挑戦する', '同じシードでもう一度', 'ダンジョンを選ぶ', '結果を見る'])
  call s:answer(4)
  call s:check('the result stays', search('冒険の結果', 'nw') > 0, 1)
  call goda_rogue#do('wait')
  call s:answer(1)
  call s:check('retried', getline(1) =~# '^1F ', 1)
  call s:check('retried: a new run', search('冒険の結果', 'nw'), 0)
  call s:check('retried: the same dungeon', s:messages(), ['1F に着いた。'])
  call s:check('retried: another seed', s:status() =~# 'シード records$', 0)

  " The same seed again: the same floors. (The debug mode ends the run.)
  call goda_rogue#start(#{dungeon: 'cave', seed: 'again'})
  let first = getline(2, line('$') - 5)
  for _ in range(3000)
    if search('冒険の結果', 'nw') > 0
      break
    endif
    call goda_rogue#do('wait')
  endfor
  call goda_rogue#do('wait')
  call s:answer(2)
  call s:check('again: the same seed', s:status() =~# 'シード again$', 1)
  call s:check('again: the same floor', getline(2, line('$') - 5), first)
  unlet g:goda_rogue#prompt_func
  call goda_rogue#records()
  call s:check('records buffer', bufname('%'), 'goda-rogue://records')
  call s:check('records', getline(1), '自己ベスト')
  call s:check('all records', search('^すべての記録（\d\+回', 'n') > 0, 1)
endfunction

" The status line {status} without its play time.
function! s:timeless(status) abort
  return substitute(a:status, '時間 \S\+', '', '')
endfunction

try
  call s:start()
  call s:actions()
  call s:records()
  call s:replays()
  call s:debug()
catch
  call add(s:errors, v:throwpoint . ': ' . v:exception)
endtry
" On a failure, the messages too: an error the plugin printed (a request
" that failed) explains a check that failed after it.
call writefile(s:errors + (empty(s:errors) ? ['ok']
      \ : ['--- messages:'] + split(execute('messages'), "\n"))
      \ + goda#log(), s:result)
call goda#stop()
qall!
