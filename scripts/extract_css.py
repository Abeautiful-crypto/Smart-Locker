import re, io, os
src = r"c:/Users/91526/Desktop/智能寄存柜/Smart-Locker/prototype/智能寄存柜-新粗野主义原型.html"
dst = r"c:/Users/91526/Desktop/智能寄存柜/Smart-Locker/web/proto.css"
os.makedirs(os.path.dirname(dst), exist_ok=True)
s = io.open(src, encoding="utf-8").read()
m = re.search(r"<style>(.*?)</style>", s, re.S)
css = m.group(1).strip()
io.open(dst, "w", encoding="utf-8", newline="\n").write(css + "\n")
print("proto.css bytes:", len(css.encode("utf-8")))
