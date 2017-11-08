#!/bin/bash

file="$1"; shift
if [[ -z "$file" ]]; then
    echo "usage: $0 demo.cfg" >&2
    exit 1
fi

source "$file"

if [[ -z "$scriptfile" ]]; then
    echo "scriptfile is empty" >&2
    exit 1
fi

if [[ -z "$output" ]]; then
    echo "output is empty" >&2
    exit 1
fi

echo "select shell window to use"
id=$(xwininfo | grep 'id: ' | cut -d' ' -f4)
if [[ -z "$id" ]]; then
    echo "failed to get window id" >&2
    exit 1
fi

xdotool windowfocus "$id"
xdotool windowsize --usehints "$id" "$width" "$height"

if [[ -e "$prescriptfile" ]]; then
    echo "executing $prescriptfile"
    xdotool type --window "$id" --file "$prescriptfile"
fi

xdotool key --window "$id" ctrl+l

sleep 1

(sleep 1; xdotool type --window "$id" --file "$scriptfile") &

if [[ -n "$waitfor" ]]; then
    echo "automatic recording finish after $waitfor seconds"
    exec 3< <(sleep "$waitfor"; echo -n q)
else
    echo "press q to finish recording"
    exec 3<&0
fi

TMP_AVI=$(mktemp /tmp/outXXXXXXXXXX.avi)
(ffcast -# $id ffmpeg -loglevel quiet \
    -y -f x11grab -show_region 1 -framerate 15 \
    -video_size %s -i %D+%c -codec:v huffyuv                  \
    -vf crop="iw-mod(iw\\,2):ih-mod(ih\\,2)" $TMP_AVI         \
&& convert -set delay 10 -layers Optimize $TMP_AVI $output) <&3
