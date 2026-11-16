# goda-rogue.vim

A mystery-dungeon roguelike in Vim and Neovim on
[goda.vim](https://github.com/Shougo/goda.vim). It looks like `rogue`, with no
images: the map is drawn with full-width characters, enemies are kanji, and
their rank is their color.

> Status: a prototype. It shows the dungeon: floors made from a seed, the
> player `主`, walls, stairs, enemies and items, moving around,
> attacking, and a mini-map. Enemies wander, chase and attack the player,
> who levels up, and can fall; 30 floors to clear. There is no item use
> yet.

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
  nnoremap <buffer> m <Cmd>call goda_rogue#do('map')<CR>
  nnoremap <buffer> M <Cmd>call goda_rogue#do('log')<CR>
endfunction
```

Then:

```vim
call goda_rogue#start()
" The same seed makes the same floors.
call goda_rogue#start(#{seed: 'vim'})
```

The binary is built on the first start (it takes a while), into a cache
directory. See `:help goda-rogue` for everything else.

## License

MIT, see [LICENSE](LICENSE).
