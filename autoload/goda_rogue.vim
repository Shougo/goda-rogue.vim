" goda-rogue.vim - a mystery-dungeon roguelike for Vim/Neovim on goda.vim.
"
" The Go side (the binary built from this repository) owns the whole game;
" this file sends it the actions and shows the screens it returns: lines,
" highlights, and the cursor on the player.

" The version of this Vim script that the binary goes with (ScriptVersion of
" its Go side): goda.vim refuses a binary of another one, and builds it again.
let s:version = #{version: 2, module: 'github.com/Shougo/goda-rogue.vim'}

" The repository: the Go module of the binary.
let s:root = expand('<sfile>:p:h:h', 1)

let s:ns = has('nvim') ? nvim_create_namespace('goda-rogue') : 0

" Highlights over the others (the background of the tile the player faces).
let s:over = #{GodaRogueFacing: 1}

" The window of the mini-map (a popup in Vim, a floating window in Neovim),
" and its buffer in Neovim.
let s:map = -1
let s:map_buf = -1

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

" The runs are kept in a data directory: the replay file of the last run
" (also the suspended run), the replays saved by goda_rogue#save_replay(),
" and those of the runs before (replays/auto).
function! s:data_dir() abort
  if exists('g:goda_rogue#data_dir')
    return expand(g:goda_rogue#data_dir, 1)
  endif
  if has('nvim')
    let base = stdpath('data')
  elseif !empty($XDG_DATA_HOME)
    let base = $XDG_DATA_HOME . '/vim'
  elseif has('win32') && !empty($LOCALAPPDATA)
    let base = $LOCALAPPDATA . '/vim'
  else
    let base = expand('~/.local/share/vim', 1)
  endif
  return base . '/goda-rogue'
endfunction

" The replay file of the last run.
function! goda_rogue#last_replay() abort
  return s:data_dir() . '/last.replay'
endfunction

function! s:replay_dir() abort
  return s:data_dir() . '/replays'
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
      \   trap: ['208', '#ff8700'],
      \   facing: ['238', '#444444'],
      \   hp: ['34', '#00af00'], hplow: ['160', '#d70000'],
      \   hpempty: ['238', '#444444'],
      \   ranks: [['77', '#5fd75f'], ['69', '#5f87ff'],
      \           ['170', '#d75fd7'], ['203', '#ff5f5f']],
      \ },
      \ light: #{
      \   wall: ['30', '#008787'], floor: ['250', '#bcbcbc'],
      \   safe: ['130', '#af5f00'], stairs: ['136', '#af8700'],
      \   trap: ['166', '#d75f00'],
      \   facing: ['253', '#dadada'],
      \   hp: ['71', '#5faf5f'], hplow: ['160', '#d70000'],
      \   hpempty: ['252', '#d0d0d0'],
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
        \ ['GodaRogueTrap', c.trap, 'bold'],
        \ ]
    call s:highlight(group, color, attr)
  endfor
  " The tile the player faces: a background over the highlight of the tile.
  call s:highlight('GodaRogueFacing', c.facing, '', 'bg')
  highlight default link GodaRogueOldMessage Comment
  highlight default link GodaRogueTitle Title
  " The HP bar of the top line: spaces with a background.
  call s:highlight('GodaRogueHP', c.hp, '', 'bg')
  call s:highlight('GodaRogueHPLow', c.hplow, '', 'bg')
  call s:highlight('GodaRogueHPEmpty', c.hpempty, '', 'bg')
  highlight default GodaRoguePlayer term=bold cterm=bold gui=bold
  for i in range(len(c.ranks))
    call s:highlight('GodaRogueRank' . (i + 1), c.ranks[i], '')
    call s:highlight('GodaRogueAlly' . (i + 1), c.ranks[i], 'underline')
    call s:highlight('GodaRogueKeeper' . (i + 1), c.ranks[i], 'reverse')
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

" The dungeons there are: [{id, name, description, floors}].
function! goda_rogue#dungeons() abort
  return goda#request('rogue', 'dungeons')
endfunction

" Start a new game in the current window. {options}:
"   dungeon	The ID of the dungeon (goda_rogue#dungeons()).  Default: ask
"		which one, with the selection UI.
"   seed	The same seed makes the same floors (a string or a number).
"		Default: a random one.
"   debug	Enables the debug mode.  Default: g:goda_rogue#debug.
" A suspended run is offered to be resumed first.
function! goda_rogue#start(options = {}) abort
  let options = copy(a:options)
  if type(get(options, 'seed', '')) == v:t_number
    let options.seed = string(options.seed)
  endif
  try
    let last = goda#request('rogue', 'last', goda_rogue#last_replay())
  catch
    call s:print_error(v:exception)
    return
  endtry
  if last.state ==# 'suspended'
    call s:select(printf('中断した冒険があります（%s %dF  %dターン  %s）',
          \ last.name, last.depth, last.turn, last.time),
          \ ['再開する', '捨てて新しく始める'],
          \ {idx -> idx == 1 ? s:resume()
          \       : idx == 2 ? s:choose_dungeon(options) : 0})
    return
  elseif last.state ==# 'broken'
    " Another version, say: it can still be saved, to watch it with the
    " version that played it.
    call s:select('再開できない冒険があります（' . last.error . '）',
          \ ['リプレイを保存して新しく始める', '捨てて新しく始める'],
          \ {idx -> idx == 1 ? s:save_and_choose(options)
          \       : idx == 2 ? s:choose_dungeon(options) : 0})
    return
  endif
  call s:choose_dungeon(options)
endfunction

function! s:save_and_choose(options) abort
  if !empty(goda_rogue#save_replay())
    call s:choose_dungeon(a:options)
  endif
endfunction

function! s:choose_dungeon(options) abort
  let options = a:options
  if has_key(options, 'dungeon')
    call s:start_game(options)
    return
  endif
  try
    let dungeons = goda_rogue#dungeons()
  catch
    call s:print_error(v:exception)
    return
  endtry
  let choices = map(copy(dungeons),
        \ {_, d -> printf('%s（%dF）  %s', d.name, d.floors, d.description)})
  call s:select('ダンジョン', choices,
        \ {idx -> idx > 0 ? s:start_game(extend(options, #{dungeon: dungeons[idx - 1].id})) : 0})
endfunction

" Show the game buffer in the current window.
function! s:open_buffer() abort
  call s:stop_watching()
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

  " The mini-map goes with the game buffer.
  autocmd! goda-rogue * <buffer>
  autocmd goda-rogue BufLeave <buffer> call s:close_map()
  autocmd goda-rogue BufEnter <buffer> call goda_rogue#do('redraw')
endfunction

function! s:start_game(options) abort
  call s:open_buffer()
  let options = extend(copy(a:options), #{replay: goda_rogue#last_replay(),
        \ keep: get(g:, 'goda_rogue#kept_replays', 50)})
  " (A boolean for the Go side, whatever was given: 1, v:true, ...)
  let options.debug = get(options, 'debug', get(g:, 'goda_rogue#debug', v:false))
        \ ? v:true : v:false
  try
    let screen = goda#request('rogue', 'start', options, s:size())
  catch
    call s:print_error(v:exception)
    return
  endtry
  call s:draw(screen)
endfunction

function! s:resume() abort
  call s:open_buffer()
  try
    let screen = goda#request('rogue', 'resume', goda_rogue#last_replay(), s:size())
  catch
    call s:print_error(v:exception)
    return
  endtry
  call s:draw(screen)
endfunction

" Suspend the run: it is kept in its replay file, and goda_rogue#start()
" offers to resume it.  The same as goda_rogue#do('suspend').
function! goda_rogue#suspend() abort
  if &filetype !=# 'goda-rogue'
    return
  endif
  try
    call goda#request('rogue', 'do', 'suspend', [], s:size())
  catch
    call s:print_error(v:exception)
    return
  endtry
  call s:close_prompt()
  call s:close_map()
  setlocal modifiable
  silent %delete _
  call setline(1, ['', '  冒険を中断した。', '',
        \ '  goda_rogue#start() で再開できる。'])
  setlocal nomodifiable
endfunction

" Save the replay of the last run as {name} (default: the date and time)
" in the directory of the replays, and return its path ('' if it could not
" be saved).
function! goda_rogue#save_replay(name = '') abort
  let src = goda_rogue#last_replay()
  if !filereadable(src)
    call s:print_error('no replay: play a run first')
    return ''
  endif
  let name = empty(a:name) ? strftime('%Y%m%d-%H%M%S') : a:name
  if name !~# '\.replay$'
    let name .= '.replay'
  endif
  call mkdir(s:replay_dir(), 'p')
  let dst = s:replay_dir() . '/' . name
  if writefile(readfile(src, 'b'), dst, 'b') != 0
    call s:print_error('could not write ' . dst)
    return ''
  endif
  echomsg '[goda-rogue] the replay is saved: ' . dst
  return dst
endfunction

" Show the records of the finished runs (番付) in the buffer
" "goda-rogue://records": the best run of each dungeon, then every run.
function! goda_rogue#records() abort
  try
    let lines = goda#request('rogue', 'records', s:data_dir() . '/records.tsv')
  catch
    call s:print_error(v:exception)
    return
  endtry
  let bufnr = bufadd('goda-rogue://records')
  call setbufvar(bufnr, '&buftype', 'nofile')
  call setbufvar(bufnr, '&bufhidden', 'hide')
  call setbufvar(bufnr, '&swapfile', 0)
  call setbufvar(bufnr, '&buflisted', 0)
  call bufload(bufnr)
  execute 'buffer' bufnr
  setlocal modifiable
  silent %delete _
  call setline(1, lines)
  setlocal nomodifiable nowrap
  setlocal filetype=goda-rogue-records
endfunction

" Watch the replay {file} (default: the last run) in the current window: it
" plays by itself, an action every g:goda_rogue#replay_interval
" milliseconds; goda_rogue#replay_pause() pauses it or plays it again, and
" goda_rogue#replay_step() steps it an action.
function! goda_rogue#replay(file = '') abort
  let file = empty(a:file) ? goda_rogue#last_replay() : expand(a:file, 1)
  call s:open_buffer()
  try
    let screen = goda#request('rogue', 'watch', fnamemodify(file, ':p'), s:size())
  catch
    call s:print_error(v:exception)
    return
  endtry
  call s:draw(screen)
  let s:watching = #{bufnr: bufnr('%'), done: v:false}
  autocmd goda-rogue BufLeave <buffer> call s:stop_watching()
  call s:play_replay()
endfunction

" The replay being watched: its buffer, whether it is over, and its timer.
let s:watching = {}
let s:replay_timer = -1

function! s:play_replay() abort
  call timer_stop(s:replay_timer)
  let s:replay_timer = timer_start(get(g:, 'goda_rogue#replay_interval', 300),
        \ {-> s:replay_tick()}, #{repeat: -1})
endfunction

function! s:replay_tick() abort
  if empty(s:watching) || s:watching.done
    call timer_stop(s:replay_timer)
    return
  endif
  call goda_rogue#replay_step()
endfunction

" Pause the replay being watched, or play it again.
function! goda_rogue#replay_pause() abort
  if empty(s:watching)
    return
  endif
  if empty(timer_info(s:replay_timer))
    call s:play_replay()
  else
    call timer_stop(s:replay_timer)
    let s:replay_timer = -1
  endif
endfunction

" Step the replay being watched {n} actions (-1: to its end).
function! goda_rogue#replay_step(n = 1) abort
  if empty(s:watching) || s:watching.done
    return
  endif
  let winids = win_findbuf(s:watching.bufnr)
  if empty(winids)
    return
  endif
  call win_execute(winids[0], 'call s:step(' . a:n . ')')
endfunction

function! s:step(n) abort
  try
    let r = goda#request('rogue', 'step', a:n, s:size())
  catch
    call s:stop_watching()
    call s:print_error(v:exception)
    return
  endtry
  let s:watching.done = r.done
  call s:draw(r.screen)
endfunction

function! s:stop_watching() abort
  call timer_stop(s:replay_timer)
  let s:replay_timer = -1
  let s:watching = {}
endfunction

" Run the action {action} of the game, with its arguments: >
"	call goda_rogue#do('move', 'left')
" Actions: move, dash and turn (with a direction: left, right, up, down,
" upleft, upright, downleft, downright), attack, shoot, wait, stairs, pickup,
" inventory, choose (an index, from 0), cancel, look, map, log, redraw,
" suspend, debug.
function! goda_rogue#do(action, ...) abort
  if &filetype !=# 'goda-rogue'
    return
  endif
  if a:action ==# 'suspend'
    call goda_rogue#suspend()
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

  call s:highlight_buffer(bufnr('%'), a:screen.highlights)
  call cursor(a:screen.cursor[0], a:screen.cursor[1])

  if has_key(a:screen, 'minimap')
    call s:draw_map(a:screen.minimap)
  else
    call s:close_map()
  endif

  if has_key(a:screen, 'log')
    call s:show_log(a:screen.log)
  endif

  if has_key(a:screen, 'prompt')
    call s:ask(a:screen.prompt)
  else
    call s:close_prompt()
  endif
endfunction

" ---------------------------------------------------------------------------
" Prompts: the choices the game asks for (which item, what to do with it).
" They are shown with the editor's selection UI, or by
" g:goda_rogue#prompt_func; the answer goes back as the action "choose" (the
" index from 0) or "cancel".

" The popup of the prompt (Vim).
let s:prompt = -1

function! s:ask(prompt) abort
  let winid = win_getid()
  call s:select(a:prompt.title, a:prompt.choices,
        \ {idx -> goda_rogue#_answer(winid, idx)})
endfunction

" Ask to choose one of {choices} with the selection UI; {callback} gets the
" choice, from 1 (0 or less for none).
function! s:select(title, choices, callback) abort
  let s:on_select = a:callback
  if !empty(get(g:, 'goda_rogue#prompt_func', ''))
    call call(g:goda_rogue#prompt_func,
          \ [#{title: a:title, choices: a:choices}, {idx -> s:selected(idx)}])
  elseif has('nvim')
    " Ask once the current action is over: an answer comes inside
    " win_execute(), where inputlist() does not wait for one.  Clear what an
    " earlier selection left on the command line first.  The default
    " vim.ui.select() is replaced by an inputlist() with the numbers aligned
    " (the items are aligned in columns); one set by a plugin is used.
    call luaeval('vim.schedule(function()'
          \ . ' vim.cmd("redraw")'
          \ . ' local done = vim.fn["goda_rogue#_selected"]'
          \ . ' local src = debug.getinfo(vim.ui.select, "S").source'
          \ . ' if not src:find("vim[/\\]ui%.lua$") then'
          \ . '  vim.ui.select(_A[1], {prompt = _A[2]},'
          \ . '   function(_, idx) done(idx or 0) end)'
          \ . '  return'
          \ . ' end'
          \ . ' local lines = {_A[2]}'
          \ . ' local format = "%" .. #tostring(#_A[1]) .. "d: %s"'
          \ . ' for i, c in ipairs(_A[1]) do'
          \ . '  lines[#lines + 1] = format:format(i, c)'
          \ . ' end'
          \ . ' local idx = vim.fn.inputlist(lines)'
          \ . ' done((idx >= 1 and idx <= #_A[1]) and idx or 0)'
          \ . ' end)',
          \ [a:choices, a:title])
  else
    call s:close_prompt()
    let s:prompt = popup_menu(a:choices, #{
          \ title: ' ' . a:title . ' ',
          \ callback: {_, result -> s:selected(result)},
          \ })
  endif
endfunction

function! goda_rogue#_selected(idx) abort
  call s:selected(a:idx)
endfunction

function! s:selected(idx) abort
  if a:idx == -2
    " A popup closed by the game: no answer.
    return
  endif
  let s:prompt = -1
  call call(s:on_select, [a:idx])
endfunction

function! s:close_prompt() abort
  if s:prompt >= 0 && !has('nvim')
    let id = s:prompt
    let s:prompt = -1
    " (Its callback then answers nothing: the prompt is already gone.)
    call popup_close(id, -2)
  endif
endfunction

" Answer the prompt shown in the window {winid} with the choice {idx}, from
" 1 (0 or less to cancel).
function! goda_rogue#_answer(winid, idx) abort
  let cmd = a:idx > 0
        \ ? printf('call goda_rogue#do("choose", "%d")', a:idx - 1)
        \ : 'call goda_rogue#do("cancel")'
  call win_execute(a:winid, cmd)
endfunction

" Show {log} (all the messages) in the buffer "goda-rogue://log", in a
" window under the game, with the cursor on the last message.
function! s:show_log(log) abort
  let bufnr = bufadd('goda-rogue://log')
  call setbufvar(bufnr, '&buftype', 'nofile')
  call setbufvar(bufnr, '&bufhidden', 'hide')
  call setbufvar(bufnr, '&swapfile', 0)
  call setbufvar(bufnr, '&buflisted', 0)
  call bufload(bufnr)
  let winids = win_findbuf(bufnr)
  if empty(winids)
    execute 'belowright 10split'
    execute 'buffer' bufnr
  else
    call win_gotoid(winids[0])
  endif
  setlocal modifiable
  silent %delete _
  call setline(1, a:log)
  setlocal nomodifiable
  setlocal filetype=goda-rogue-log
  normal! G
endfunction

" Set {highlights} ({group: spans}) of the Go side in the buffer {bufnr}.
function! s:highlight_buffer(bufnr, highlights) abort
  if has('nvim')
    call nvim_buf_clear_namespace(a:bufnr, s:ns, 0, -1)
    for [group, spans] in items(a:highlights)
      for s in spans
        call nvim_buf_set_extmark(a:bufnr, s:ns, s.line - 1, s.col - 1,
              \ #{end_col: s.end - 1, hl_group: group,
              \   priority: has_key(s:over, group) ? 4200 : 4096})
      endfor
    endfor
    return
  endif
  call prop_clear(1, getbufinfo(a:bufnr)[0].linecount, #{bufnr: a:bufnr})
  for [group, spans] in items(a:highlights)
    if empty(prop_type_get(group))
      call prop_type_add(group, #{highlight: group, combine: v:true,
            \ priority: has_key(s:over, group) ? 10 : 0})
    endif
    call prop_add_list(#{type: group, bufnr: a:bufnr},
          \ map(copy(spans), {_, s -> [s.line, s.col, s.line, s.end]}))
  endfor
endfunction

" Show {minimap} in the corner of the current window (the game's) away from
" the player (the cursor), to cover as little around it as possible.
function! s:draw_map(minimap) abort
  let lines = a:minimap.lines
  let info = getwininfo(win_getid())[0]
  let right = wincol() <= info.width / 2
  let bottom = winline() <= info.height / 2
  if has('nvim')
    if s:map_buf < 0 || !nvim_buf_is_valid(s:map_buf)
      let s:map_buf = nvim_create_buf(v:false, v:true)
    endif
    call nvim_buf_set_lines(s:map_buf, 0, -1, v:true, lines)
    " (Placed again every time: the window may have been resized.)
    let config = #{
          \ relative: 'win', win: win_getid(),
          \ anchor: (bottom ? 'S' : 'N') . (right ? 'E' : 'W'),
          \ row: bottom ? info.height : 0, col: right ? info.width : 0,
          \ width: strwidth(lines[0]), height: len(lines),
          \ border: 'single', title: ' map ',
          \ }
    if s:map < 0 || !nvim_win_is_valid(s:map)
      let s:map = nvim_open_win(s:map_buf, v:false, extend(config,
            \ #{style: 'minimal', focusable: v:false, zindex: 50}))
    else
      call nvim_win_set_config(s:map, config)
    endif
    call s:highlight_buffer(s:map_buf, a:minimap.highlights)
    return
  endif
  let pos = #{
        \ line: bottom ? info.winrow + info.height - 1 : info.winrow,
        \ col: right ? info.wincol + info.width - 1 : info.wincol,
        \ pos: (bottom ? 'bot' : 'top') . (right ? 'right' : 'left'),
        \ }
  if s:map < 0 || empty(popup_getpos(s:map))
    let s:map = popup_create(lines, extend(#{
          \ border: [], title: ' map ', highlight: 'Normal', zindex: 50,
          \ }, pos))
  else
    call popup_settext(s:map, lines)
    call popup_move(s:map, pos)
  endif
  call s:highlight_buffer(winbufnr(s:map), a:minimap.highlights)
endfunction

function! s:close_map() abort
  if s:map < 0
    return
  endif
  if has('nvim')
    if nvim_win_is_valid(s:map)
      call nvim_win_close(s:map, v:true)
    endif
  else
    call popup_close(s:map)
  endif
  let s:map = -1
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
