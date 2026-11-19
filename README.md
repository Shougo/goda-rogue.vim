<p align="center">
  <img src="assets/logo.svg" alt="goda-rogue.vim: a room of the game, 主 among enemies in the colors of their ranks">
</p>

# goda-rogue.vim

A mystery-dungeon roguelike in Vim and Neovim on
[goda.vim](https://github.com/Shougo/goda.vim). It looks like `rogue`, with no
images: the map is drawn with full-width characters, enemies are kanji, and
their rank is their color.

```
＃＃＃＃＃＃＃＃＃＃＃＃＃
＃．．．．．．．．鼠．．＃　　主  you
＃．．主．．．．．．．．．　　段  the stairs down
＃．．．．草．．．蛇．．＃　　草  an item (a grass)
＃．鬼．．．．．．．．段＃　　鼠 蛇 鬼 竜  enemies: green, blue,
＃．．．．．．竜．．．．＃　　            purple, red by rank
＃＃＃＃＃＃＃＃＃＃＃＃＃
```

> Status: a prototype. It shows the dungeon: floors made from a seed, the
> player `主`, walls, stairs, enemies and items, moving around,
> attacking, and a mini-map. Enemies wander, chase and attack the player,
> who levels up, and can fall; 30 floors to clear. Items can be picked
> up and dropped, weapons, shields and bracelets equipped, and grasses,
> scrolls, staffs and food used, arrows shot, items thrown and
> identified; the player gets hungry, and finds traps by swinging at
> nothing. Pots keep and merge items; weapons and shields have seals.
> Some floors are a big room, a maze, or have a monster house or a shop
> (buy, sell, or steal). A run can be suspended and resumed, and watched
> again as a replay.

## Requirements

- Vim 9.1 or Neovim 0.11
- Go 1.26 (to build the binary)
- [goda.vim](https://github.com/Shougo/goda.vim)

## Usage

There are no default mappings: map the actions in the game buffer (its
filetype is `goda-rogue`):

```vim
autocmd FileType goda-rogue call s:goda_rogue_settings()
function s:goda_rogue_settings() abort
  const dirs = #{h: 'left', j: 'down', k: 'up', l: 'right',
        \ y: 'upleft', u: 'upright', b: 'downleft', n: 'downright'}
  for [key, dir] in items(dirs)
    execute $'nnoremap <buffer> {key} <Cmd>call goda_rogue#do("move", "{dir}")<CR>'
    execute $'nnoremap <buffer> g{key} <Cmd>call goda_rogue#do("turn", "{dir}")<CR>'
    execute $'nnoremap <buffer> {key->toupper()} <Cmd>call goda_rogue#do("dash", "{dir}")<CR>'
  endfor
  nnoremap <buffer> <Space> <Cmd>call goda_rogue#do('attack')<CR>
  " Face an enemy next to the player (again: the next one).
  nnoremap <buffer> t <Cmd>call goda_rogue#do('turn')<CR>
  nnoremap <buffer> . <Cmd>call goda_rogue#do('wait')<CR>
  nnoremap <buffer> > <Cmd>call goda_rogue#do('stairs')<CR>
  nnoremap <buffer> i <Cmd>call goda_rogue#do('inventory')<CR>
  nnoremap <buffer> , <Cmd>call goda_rogue#do('pickup')<CR>
  nnoremap <buffer> f <Cmd>call goda_rogue#do('shoot')<CR>
  nnoremap <buffer> x <Cmd>call goda_rogue#do('look')<CR>
  nnoremap <buffer> m <Cmd>call goda_rogue#do('map')<CR>
  nnoremap <buffer> I <Cmd>call goda_rogue#do('items')<CR>
  nnoremap <buffer> M <Cmd>call goda_rogue#do('log')<CR>
  nnoremap <buffer> S <Cmd>call goda_rogue#suspend()<CR>
endfunction
```

Then:

```vim
" Choose a dungeon, and start (or resume a suspended run).
call goda_rogue#start()
" The same seed makes the same floors.
call goda_rogue#start(#{dungeon: 'cave', seed: 'vim'})
" Keep the replay of the last run, and watch it.
call goda_rogue#replay(goda_rogue#save_replay())
" The records of the finished runs, the best first.
call goda_rogue#records()
```

The binary is built on the first start (it takes a while), into a cache
directory. See `:help goda-rogue` for everything else.

## Dungeons

Dungeons are data: tab-separated text files in `rogue/dungeons/{id}/`
(monsters, which floors they appear on, items, rules, growth tables).
See `:help goda-rogue-dungeons`.

To see how a dungeon plays, let bots play it many times:

```sh
go run ./cmd/analyze -dungeon cave -runs 200
```

## License

MIT, see [LICENSE](LICENSE).
