#!/usr/bin/env python3
"""Gera emojis.tsv a partir do emoji-test.txt (Unicode) e das anotações do CLDR (pt/en).
Uso: python3 gen.py <emoji-test.txt> <pt.xml> <en.xml> [fonte.ttf] > emojis.tsv
Emojis que a fonte não cobre (versões novas do Unicode) ficam de fora: virariam um
quadradinho vazio. Padrão: /usr/share/fonts/noto/NotoColorEmoji.ttf.
Colunas: emoji, grupo, nome pt, nome en, palavras-chave (pt + en)."""
import sys
import xml.etree.ElementTree as ET

from fontTools.ttLib import TTFont


def load(path):
    names, keys = {}, {}
    for a in ET.parse(path).getroot().iter("annotation"):
        cp = a.get("cp")
        if a.get("type") == "tts":
            names[cp] = (a.text or "").strip()
        else:
            keys[cp] = [k.strip() for k in (a.text or "").split("|")]
    return names, keys


def look(d, cp):
    return d.get(cp) or d.get(cp.replace("️", "")) or d.get(cp + "️")


FONT = sys.argv[4] if len(sys.argv) > 4 else "/usr/share/fonts/noto/NotoColorEmoji.ttf"
cmap = TTFont(FONT, lazy=True).getBestCmap()
JOINERS = {0xFE0F, 0x200D, 0x20E3}  # sem glifo próprio no cmap


def supported(emoji):
    return all(ord(c) in cmap for c in emoji if ord(c) not in JOINERS)


pt_n, pt_k = load(sys.argv[2])
en_n, en_k = load(sys.argv[3])
group = ""
for line in open(sys.argv[1], encoding="utf8"):
    if line.startswith("# group:"):
        group = line.split(":", 1)[1].strip()
        continue
    if not line.strip() or line[0] == "#" or group == "Component":
        continue
    meta, _, comment = line.partition("#")
    if meta.split(";")[1].strip() != "fully-qualified":
        continue
    emoji, _, rest = comment.strip().partition(" ")
    en = rest.split(" ", 1)[1]
    if "skin tone" in en:  # variantes de tom de pele ficam de fora
        continue
    if not supported(emoji):
        continue
    pt = look(pt_n, emoji) or en
    kw = dict.fromkeys((look(pt_k, emoji) or []) + (look(en_k, emoji) or []))
    print("\t".join([emoji, group, pt, en, " ".join(kw)]))
