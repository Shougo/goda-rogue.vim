" goda-rogue.vim - a mystery-dungeon roguelike for Vim/Neovim on goda.vim.
"
" The Go side (the binary built from this repository) owns the whole game;
" this file sends it the actions and shows the screens it returns: lines,
" highlights, and the cursor on the player.

" The version of this Vim script that the binary goes with (ScriptVersion of
" its Go side): goda.vim refuses a binary of another one, and builds it again.
let s:version = #{version: 5, module: 'github.com/Shougo/goda-rogue.vim'}

" The repository: the Go module of the binary.
let s:root = expand('<sfile>:p:h:h', 1)

let s:ns = has('nvim') ? nvim_create_namespace('goda-rogue') : 0

" Highlights over the others (the background of the tile the player faces).
let s:over = #{GodaRogueFacing: 1}

" The windows beside the game (a popup in Vim, a floating window in Neovim):
" the mini-map ("map") and the items carried ("items"), by name, and their
" buffers in Neovim.
let s:floats = {}
let s:float_bufs = {}

" ---------------------------------------------------------------------------
" Setup

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

" The replay file of the last run a bot played (goda_rogue#watch_bot()).
function! goda_rogue#bot_replay() abort
  return s:replay_dir() . '/bot.replay'
endfunction

function! s:replay_dir() abort
  return s:data_dir() . '/replays'
endfunction

" The binary is built from this repository by goda.vim, into the cache
" directory, when there is none or it is of another version; with
" g:goda_rogue#binary the user builds it, and g:goda_rogue#config_dir is a
" copy of the repository to build instead (other dungeons, say). The binary
" has no "status" to ask: it has no configuration that can be invalid.
let s:goda = goda#configured#new('rogue',
      \ extend(#{dir: s:root, status: v:false}, s:version))

" Build the binary again (after an update of this plugin, say). The running
" one is stopped, and the new one starts with the next request.
function! goda_rogue#build() abort
  call s:goda.build()
  call s:goda.register()
endfunction

call s:goda.register()

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
  highlight default link GodaRogueHungry WarningMsg
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
  " An enemy beyond rank 4: the color of rank 4, reversed and bold.
  call s:highlight('GodaRogueRankOver', c.ranks[-1], 'reverse,bold')
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
        \ {_, d -> printf('%s（%dF）', d.name, d.floors)})
  call s:select('ダンジョン', choices,
        \ {idx -> idx > 0 ? s:dungeon_menu(options, dungeons[idx - 1]) : 0})
endfunction

" What to do with the dungeon {dungeon} (an entry of goda_rogue#dungeons()):
" play it, read its description, watch a bot play it, or leave it.
function! s:dungeon_menu(options, dungeon) abort
  let options = extend(copy(a:options), #{dungeon: a:dungeon.id})
  call s:select(printf('%s（%dF）', a:dungeon.name, a:dungeon.floors),
        \ ['冒険に出る', '説明を見る', 'Bot の冒険を見る', 'やめる'],
        \ {idx -> idx == 1 ? s:start_game(options)
        \       : idx == 2 ? s:describe_dungeon(a:options, a:dungeon)
        \       : idx == 3 ? s:choose_bot(options) : 0})
endfunction

" Ask which bot plays, and watch it.
function! s:choose_bot(options) abort
  try
    let bots = goda#request('rogue', 'bots')
  catch
    call s:print_error(v:exception)
    return
  endtry
  call s:select('Bot', map(copy(bots), {_, b -> b.name}),
        \ {idx -> idx > 0
        \   ? goda_rogue#watch_bot(extend(copy(a:options), #{style: bots[idx - 1].style}))
        \   : 0})
endfunction

" Show the description of {dungeon}, then its menu again.
function! s:describe_dungeon(options, dungeon) abort
  call s:select(printf('%s（%dF）： %s',
        \ a:dungeon.name, a:dungeon.floors, a:dungeon.description),
        \ ['戻る'],
        \ {idx -> s:dungeon_menu(a:options, a:dungeon)})
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
  autocmd goda-rogue BufLeave <buffer> call s:close_floats()
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
  call s:close_floats()
  setlocal modifiable
  silent %delete _
  call setline(1, ['', '  冒険を中断した。', '',
        \ '  goda_rogue#start() で再開できる。'])
  setlocal nomodifiable
endfunction

" Save the replay of the last run as {name} (default: the date and time)
" in the directory of the replays, and return its path ('' if it could not
" be saved).  {file} is the replay to save instead of that of the last run:
" goda_rogue#bot_replay(), say.
function! goda_rogue#save_replay(name = '', file = '') abort
  let src = empty(a:file) ? goda_rogue#last_replay() : expand(a:file, 1)
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
  if fnamemodify(dst, ':p') ==# fnamemodify(src, ':p')
    call s:print_error('the replay is ' . dst . ' already: another name')
    return ''
  endif
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

" Compare the replay {file} (default: the last run) with a bot of {style}
" (default: "careful"), in the buffer "goda-rogue://compare": the decisions
" where the bot would have done another kind of thing than the player (a
" step in another direction is the same kind), how often each, and each
" one with the state it was taken in.
function! goda_rogue#compare_bot(file = '', style = '') abort
  let file = empty(a:file) ? goda_rogue#last_replay() : expand(a:file, 1)
  try
    let lines = goda#request('rogue', 'compare', file, a:style)
  catch
    call s:print_error(v:exception)
    return
  endtry
  let bufnr = bufadd('goda-rogue://compare')
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

" Watch a bot play a new run in the current window, as a replay is watched:
" it plays by itself, an action every g:goda_rogue#replay_interval
" milliseconds; goda_rogue#replay_pause() and goda_rogue#replay_step() pause
" and step it.  The run is the bot's: it has no record, its replay goes to
" goda_rogue#bot_replay() (those of the runs before are kept beside it, in
" "bot"), and a suspended run is left as it is.  {options}:
"   dungeon	The ID of the dungeon.  Default: "cave".
"   seed	As in goda_rogue#start().
"   style	How the bot plays: "careful" (the default) or "reckless".
function! goda_rogue#watch_bot(options = {}) abort
  let options = {}
  for key in ['dungeon', 'seed', 'style']
    if has_key(a:options, key)
      let options[key] = type(a:options[key]) == v:t_number
            \ ? string(a:options[key]) : a:options[key]
    endif
  endfor
  let options.replay = goda_rogue#bot_replay()
  let options.keep = get(g:, 'goda_rogue#kept_bot_replays', 10)
  call s:open_buffer()
  try
    let screen = goda#request('rogue', 'bot', options, s:size())
  catch
    call s:print_error(v:exception)
    return
  endtry
  call s:draw(screen)
  let s:watching = #{bufnr: bufnr('%'), done: v:false}
  autocmd goda-rogue BufLeave <buffer> call s:stop_watching()
  call s:play_replay()
endfunction

" The replay or the bot being watched: its buffer, whether it is over, and
" its timer.
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
" inventory, foot, choose (an index, from 0), cancel, look, map, items, log, redraw,
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
  " The run is over, and the player chose what comes next.
  let next = get(screen, 'next', '')
  if next ==# 'retry'
    let options = #{dungeon: screen.dungeon}
    if has_key(screen, 'seed')
      let options.seed = screen.seed
    endif
    call goda_rogue#start(options)
  elseif next ==# 'dungeons'
    call goda_rogue#start()
  endif
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
  " The map fits in the window: no horizontal scroll for the cursor near
  " its edge (the user's 'sidescrolloff', which a :set also gives this
  " window), and the lines from their start (the status line is longer
  " than the window).
  if &l:sidescrolloff != 0
    setlocal sidescrolloff=0
  endif
  call cursor(a:screen.cursor[0], a:screen.cursor[1])
  if winsaveview().leftcol != 0
    call winrestview(#{leftcol: 0})
  endif

  call s:draw_floats(a:screen)

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
  if get(a:prompt, 'input', v:false)
    call s:input_text(a:prompt, winid)
    return
  endif
  if get(a:prompt, 'multi', v:false)
    call s:select_many(a:prompt.title, a:prompt.choices,
          \ {idx -> goda_rogue#_answer(winid, idx)})
    return
  endif
  call s:select(a:prompt.title, a:prompt.choices,
        \ {idx -> goda_rogue#_answer(winid, idx)})
endfunction

" Ask for a text ({prompt}.input: a name), starting with {prompt}.default:
" with g:goda_rogue#prompt_func (#{title, input: v:true, default}, whose
" callback gets the text, or v:null to cancel), else on the command line.
" The answer goes back as the action "name", or "cancel".
function! s:input_text(prompt, winid) abort
  let default = get(a:prompt, 'default', '')
  if !empty(get(g:, 'goda_rogue#prompt_func', ''))
    call call(g:goda_rogue#prompt_func,
          \ [#{title: a:prompt.title, input: v:true, default: default},
          \ {text -> goda_rogue#_answer_text(a:winid, text)}])
    return
  endif
  call s:close_prompt()
  " Once the current action is over: input() does not wait for an answer
  " inside win_execute().
  call timer_start(0, {-> goda_rogue#_answer_text(a:winid,
        \ s:read_text(a:prompt.title, default))})
endfunction

" The text typed after {title} (v:null when cancelled with <Esc>).
function! s:read_text(title, default) abort
  redraw
  let cancel = "\x01"
  let text = inputdialog(a:title . ': ', a:default, cancel)
  return text ==# cancel ? v:null : text
endfunction

" Answer the text prompt shown in the window {winid} with {text} (v:null
" to cancel).
function! goda_rogue#_answer_text(winid, text) abort
  let cmd = a:text is v:null
        \ ? 'call goda_rogue#do("cancel")'
        \ : printf('call goda_rogue#do("name", %s)', string(a:text))
  call win_execute(a:winid, cmd)
endfunction

" Ask to choose some of {choices}: their numbers separated by commas
" ("1,3,4"), typed on the command line under the list (any selection UI
" chooses one only).  {callback} gets the choices, from 1, as a |List| (a
" Number from g:goda_rogue#prompt_func choosing one; 0 or [] for none).
function! s:select_many(title, choices, callback) abort
  let s:on_select = a:callback
  if !empty(get(g:, 'goda_rogue#prompt_func', ''))
    call call(g:goda_rogue#prompt_func,
          \ [#{title: a:title, choices: a:choices, multi: v:true},
          \ {idx -> s:selected(idx)}])
    return
  endif
  call s:close_prompt()
  " Once the current action is over: input() does not wait for an answer
  " inside win_execute().
  call timer_start(0, {-> s:input_many(a:title, a:choices)})
endfunction

function! s:input_many(title, choices) abort
  let answer = s:input_choices(a:title, a:choices, '番号（カンマで区切って複数）: ')
  let idx = []
  for f in split(answer, '\s*,\s*')
    let n = str2nr(trim(f))
    if n >= 1 && n <= len(a:choices) && index(idx, n) < 0
      call add(idx, n)
    endif
  endfor
  call s:selected(idx)
endfunction

" Show {choices} under {title}, numbered, in a floating window (Neovim) or
" a popup (Vim), and ask on the command line with {prompt}: the answer
" typed.  The list stays out of the messages: printed there, a long one
" scrolled the screen, and a redraw broke it (Shougo, 2026-11-24).
function! s:input_choices(title, choices, prompt) abort
  let width = len(len(a:choices))
  let lines = []
  for i in range(len(a:choices))
    call add(lines, printf('%' . width . 'd: %s', i + 1, a:choices[i]))
  endfor
  let w = min([max(map(copy(lines), 'strdisplaywidth(v:val)') + [strdisplaywidth(a:title) + 2]), &columns - 4])
  let h = min([len(lines), &lines - 5])
  if has('nvim')
    let buf = nvim_create_buf(v:false, v:true)
    call nvim_buf_set_lines(buf, 0, -1, v:true, lines)
    let win = nvim_open_win(buf, v:false, #{relative: 'editor',
          \ row: max([(&lines - h) / 2 - 1, 0]), col: max([(&columns - w) / 2, 0]),
          \ width: w, height: h, style: 'minimal', border: 'single',
          \ title: ' ' . a:title . ' ', title_pos: 'center', focusable: v:false})
  else
    let win = popup_create(lines, #{title: ' ' . a:title . ' ', border: [],
          \ pos: 'center', maxheight: h, minwidth: w})
  endif
  redraw
  try
    return input(a:prompt)
  finally
    if has('nvim')
      call nvim_win_close(win, v:true)
      call nvim_buf_delete(buf, #{force: v:true})
    else
      call popup_close(win)
    endif
    redraw
  endtry
endfunction

" Ask to choose one of {choices} on the command line, with the list in a
" window (s:input_choices): the choice, from 1 (0 for none).
function! s:input_one(title, choices) abort
  let n = str2nr(trim(s:input_choices(a:title, a:choices, '番号（q か空でやめる）: ')))
  return n >= 1 && n <= len(a:choices) ? n : 0
endfunction

" Ask to choose one of {choices} with the selection UI; {callback} gets the
" choice, from 1 (0 or less for none).
function! s:select(title, choices, callback) abort
  let s:on_select = a:callback
  if !empty(get(g:, 'goda_rogue#prompt_func', ''))
    call call(g:goda_rogue#prompt_func,
          \ [#{title: a:title, choices: a:choices}, {idx -> s:selected(idx)}])
  elseif has('nvim') && luaeval('not debug.getinfo(vim.ui.select, "S").source:find("vim[/\\]ui%.lua$")')
    " A vim.ui.select() set by a plugin.  Ask once the current action is
    " over: an answer comes inside win_execute(), where it does not wait.
    call luaeval('vim.schedule(function()'
          \ . ' vim.ui.select(_A[1], {prompt = _A[2]},'
          \ . '  function(_, idx) vim.fn["goda_rogue#_selected"](idx or 0) end)'
          \ . ' end)',
          \ [a:choices, a:title])
  elseif has('nvim')
    " Once the current action is over, as above.  The default
    " vim.ui.select() prints the list as messages: a long one broke.
    call timer_start(0, {-> s:selected(s:input_one(a:title, a:choices))})
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
  if type(a:idx) == v:t_number && a:idx == -2
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
" 1 (0 or less to cancel), or the choices of the |List| {idx} ([] to
" cancel).
function! goda_rogue#_answer(winid, idx) abort
  let idx = type(a:idx) == v:t_list ? a:idx : a:idx > 0 ? [a:idx] : []
  let cmd = !empty(idx)
        \ ? printf('call goda_rogue#do("choose", "%s")',
        \   join(map(copy(idx), 'v:val - 1'), ','))
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

" Show the mini-map and the items of {screen} in corners of the current
" window (the game's) away from the player (the cursor), to cover as little
" around it as possible: on the side away from it, the mini-map in the corner
" away from it, the items in the other one (or in that one, alone).
function! s:draw_floats(screen) abort
  let info = getwininfo(win_getid())[0]
  let right = wincol() <= info.width / 2
  let bottom = winline() <= info.height / 2
  if has_key(a:screen, 'minimap')
    call s:draw_float('map', a:screen.minimap.lines, ' map ',
          \ a:screen.minimap.highlights, bottom, right)
  else
    call s:close_float('map')
  endif
  if has_key(a:screen, 'items')
    let items = a:screen.items
    call s:draw_float('items', items.lines, ' ' . items.title . ' ', {},
          \ has_key(a:screen, 'minimap') ? !bottom : bottom, right)
  else
    call s:close_float('items')
  endif
endfunction

" The lines of the screen over the map (the top line) and under it (the
" status line and the messages), which the windows beside the game leave
" uncovered.
let s:over_map = 1
let s:under_map = 4

" Show the window {name} with {lines}, {title} and {highlights} in a corner
" of the map in the current window.
function! s:draw_float(name, lines, title, highlights, bottom, right) abort
  let winid = get(s:floats, a:name, -1)
  let info = getwininfo(win_getid())[0]
  " (The rows of the top and the bottom of the map, from 0.)
  let top = s:over_map
  let bottom = max([info.height - s:under_map, top + 1])
  " The window covers whole tiles of the map (two cells each), so that
  " its border is never over half of one: some terminals then clear the
  " whole tile, the border with it. A tile starts on an even column of
  " the text, which a horizontal scroll shifts.
  let width = max(map(copy(a:lines), 'strwidth(v:val)') + [strwidth(a:title)])
  " (With its border, an even number of cells.)
  let width += width % 2
  let shift = winsaveview().leftcol % 2
  if a:right
    let edge = info.width
    if (edge - info.textoff + shift) % 2 != 0
      let edge -= 1
    endif
    let left = max([edge - (width + 2), 0])
  else
    let left = info.textoff + shift
  endif
  if has('nvim')
    let buf = get(s:float_bufs, a:name, -1)
    if buf < 0 || !nvim_buf_is_valid(buf)
      let buf = nvim_create_buf(v:false, v:true)
      let s:float_bufs[a:name] = buf
    endif
    call nvim_buf_set_lines(buf, 0, -1, v:true, a:lines)
    " (Placed again every time: the window may have been resized.)
    let config = #{
          \ relative: 'win', win: win_getid(),
          \ anchor: (a:bottom ? 'S' : 'N') . 'W',
          \ row: a:bottom ? bottom : top, col: left,
          \ width: width,
          \ height: len(a:lines),
          \ border: 'single', title: a:title,
          \ }
    if winid < 0 || !nvim_win_is_valid(winid)
      " (noautocmd: opening it leaves the game buffer for a moment, and
      " leaving it closes the windows.)
      let s:floats[a:name] = nvim_open_win(buf, v:false, extend(config,
            \ #{style: 'minimal', focusable: v:false, zindex: 50, noautocmd: v:true}))
    else
      call nvim_win_set_config(winid, config)
    endif
    " Not see-through: with 'winblend' (the user's), the map shows in
    " its empty cells, and wide tiles break its border.
    call nvim_set_option_value('winblend', 0, #{win: s:floats[a:name]})
    call s:highlight_buffer(buf, a:highlights)
    return
  endif
  let pos = #{
        \ line: a:bottom ? info.winrow + bottom - 1 : info.winrow + top,
        \ col: info.wincol + left,
        \ pos: (a:bottom ? 'bot' : 'top') . 'left',
        \ minwidth: width, maxwidth: width,
        \ }
  if winid < 0 || empty(popup_getpos(winid))
    let winid = popup_create(a:lines, extend(#{
          \ border: [], title: a:title, highlight: 'Normal', zindex: 50,
          \ }, pos))
    let s:floats[a:name] = winid
  else
    call popup_settext(winid, a:lines)
    call popup_setoptions(winid, #{title: a:title})
    call popup_move(winid, pos)
  endif
  call s:highlight_buffer(winbufnr(winid), a:highlights)
endfunction

function! s:close_float(name) abort
  if !has_key(s:floats, a:name)
    return
  endif
  let winid = remove(s:floats, a:name)
  if has('nvim')
    if nvim_win_is_valid(winid)
      call nvim_win_close(winid, v:true)
    endif
  else
    call popup_close(winid)
  endif
endfunction

function! s:close_floats() abort
  for name in keys(s:floats)
    call s:close_float(name)
  endfor
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
