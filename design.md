# Tại Sao Tôi Build Lại Git Từ Đầu

> Đây không phải tài liệu kỹ thuật. Đây là câu chuyện về lý do tôi quyết định hiểu Git thật sự, thay vì chỉ biết dùng nó.

---

## Vấn đề bắt đầu từ giảng đường

Có một nghịch lý phổ biến trong nhiều trường đại học kỹ thuật: Git không được dạy chính thức, nhưng lại được yêu cầu sử dụng trong gần như mọi môn học có thực hành nhóm.

Kết quả là sinh viên tự mò. Và khi tự mò mà không có nền tảng, người ta học được *thao tác* nhưng không học được *tư duy*.

Biểu hiện rõ nhất là trong những buổi làm việc nhóm trên GitHub hay GitLab — khi có vấn đề xảy ra (conflict, mất code, history rối), không ai thực sự biết chuyện gì đang xảy ra bên trong. Thay vào đó, nhóm bắt đầu bàn luận, mỗi người đưa ra một giải pháp khác nhau, và cuối cùng chọn giải pháp của người *có uy tín nhất trong nhóm* — không phải giải pháp *đúng nhất về mặt kỹ thuật*.

Đây là dấu hiệu kinh điển của một nhóm đang hoạt động dựa trên niềm tin thay vì kiến thức. Và tôi đã từng là một phần của nhóm đó (HN26_CPL_FU_03 (FE)) .

---

## Cái bẫy của "biết dùng"

Trước dự án này, tôi cũng dùng được Git. `git add`, `git commit`, `git push`, `git pull`, thỉnh thoảng `git merge` hay `git rebase` khi cần. Đủ để hoàn thành công việc được giao.

Nhưng "biết dùng" và "hiểu" là hai thứ khác nhau hoàn toàn.

Khi chỉ biết dùng, người ta học theo pattern: *gặp tình huống A thì gõ lệnh B*. Pattern đó hoạt động tốt cho đến khi gặp tình huống A' — gần giống A nhưng không hoàn toàn giống. Lúc đó người ta bắt đầu đoán, bắt đầu thử, bắt đầu hỏi Stack Overflow và copy-paste giải pháp mà không hiểu tại sao nó hoạt động.

Tôi nhận ra rằng dù có thành thục đến mức nào mà không hiểu bản chất, thì vẫn sẽ thiệt về sau. Thành thục mà rỗng bên trong chỉ là một cái bẫy tinh vi hơn — vì nó tạo ra ảo giác hiểu biết.

---

## Tại sao chọn cách build lại từ đầu

Có nhiều cách để hiểu Git sâu hơn: đọc sách, xem video, đọc documentation. Tôi chọn cách build lại từ đầu vì một lý do đơn giản: *không có gì chứng minh bạn hiểu một thứ tốt hơn là tự tạo ra nó*.

Khi đọc về Content-Addressable Storage, bạn có thể gật đầu và tưởng mình hiểu. Nhưng chỉ khi tự viết hàm `WriteObject` — tự tính SHA-1, tự build header `"blob 6\0hello\n"`, tự zlib compress và lưu vào `objects/d3/48c7...` — thì lúc đó bạn mới thật sự hiểu tại sao thay đổi 1 byte lại tạo ra một object hoàn toàn khác.

Đây không phải cách học nhanh. Nhưng là cách học chắc.

---

## Ba concept — từ mơ hồ đến rõ ràng

### Content-Addressable Storage (CAS)

Trước dự án: biết Git dùng hash, nhưng không hiểu tại sao và hash được tính từ cái gì.

Sau dự án: hiểu rằng mọi thứ trong Git — file, thư mục, commit, tag, thậm chí stash — đều được lưu dưới dạng object, và mỗi object được định danh bằng **SHA-1 của chính content của nó**, không phải bằng tên file hay ID tự đặt.

Format object:
```
<type> <size>\0<content>
```

Hệ quả trực tiếp: nếu 1000 file trong project có cùng nội dung, Git chỉ lưu **1 object duy nhất**. Không cần logic deduplication phức tạp — đó là tính chất tự nhiên của content-addressable storage. Hash giống nhau → object giống nhau → không lưu lại.

Khoảnh khắc hiểu thật sự: khi chạy `mygit hash-object main.py` và nhận ra kết quả giống hệt hash trong `cat-file -p <tree>`. Không phải ngẫu nhiên — đó là *định nghĩa* của CAS.

---

### Merkle Tree

Trước dự án: biết Git lưu "snapshot" không phải "diff", nhưng không hiểu snapshot được tổ chức như thế nào.

Sau dự án: hiểu rằng mỗi commit trỏ tới một **tree object** — tree object chứa hash của các blob (file) và hash của các subtree (thư mục con). Hash của tree cha **phụ thuộc vào hash của mọi thứ bên dưới**.

Hệ quả: nếu bạn thay đổi 1 dòng trong `src/utils.py`:
- Blob của `utils.py` → hash mới
- Tree của thư mục `src/` → hash mới (vì child thay đổi)
- Tree root → hash mới (vì child thay đổi)
- Commit → hash mới

Nhưng tất cả các file khác không thay đổi → blob cũ được tái sử dụng hoàn toàn.

Chứng minh bằng số liệu thật trong dự án: commit 2 thêm `utils.py` mới và sửa `main.py`, nhưng `README.md` và `.mygitignore` không đổi. Kết quả: **4 objects mới** thay vì 6 — 2 blob cũ được tái sử dụng.

```
README.md blob ở commit 1: fbf1e1ed5af23b6ff899dd10848aabf12f100f22
README.md blob ở commit 2: fbf1e1ed5af23b6ff899dd10848aabf12f100f22
```

Cùng hash — cùng object — không lưu lại lần nào nữa.

---

### DAG of Commits

Trước dự án: nghĩ Git history là một chuỗi thẳng các commit.

Sau dự án: hiểu rằng history là **Directed Acyclic Graph** — đồ thị có hướng không có chu trình. Mỗi commit trỏ tới một hoặc nhiều *parent commits*. Merge commit trỏ tới **2 parents** — đây là điểm làm cho history trở thành graph thật sự, không phải chuỗi.

Chứng minh bằng output thật:

```
cat-file -p 457a736
tree 6121b02...
parent 757d7bf...   ← parent 1: nhánh main
parent 3a95bab...   ← parent 2: nhánh feature
author HLV ...
Merge branch 'feature-logging' into main
```

Hệ quả quan trọng: vì history là DAG, các thuật toán trên Git đều là **graph algorithms**:
- `log` = DFS/BFS từ HEAD theo parent pointers
- `merge` = tìm LCA (Lowest Common Ancestor) của 2 commits
- `bisect` = binary search trên DAG
- `rebase` = tách commit khỏi DAG và replay lên điểm mới

Khoảnh khắc hiểu thật sự: khi bisect tìm ra đúng commit gây bug trong 3 bước từ 12 commits. Không phải may mắn — đó là binary search O(log n) trên DAG.

---

## Điều tôi thật sự học được

Không phải là "biết thêm nhiều lệnh Git hơn".

Điều tôi học được là **tư duy đằng sau thiết kế**. Khi hiểu tại sao Git dùng CAS, tôi không cần ghi nhớ rằng "amend tạo commit mới" — tôi *suy ra được* điều đó vì nếu message thay đổi thì content thay đổi thì hash thay đổi thì phải là object mới. Không cần học thuộc, chỉ cần hiểu nguyên lý.

Đây là sự khác biệt giữa **kiến thức** và **hiểu biết**. Kiến thức là tập hợp các fact có thể quên. Hiểu biết là cấu trúc tư duy cho phép bạn *suy ra* facts khi cần.

Và lần sau trong nhóm khi có conflict hay history rối, tôi sẽ không đoán hay bầu theo uy tín. Tôi sẽ biết chính xác chuyện gì đang xảy ra bên trong.

---

## Ghi chú cuối

Dự án này không phải clone Git hoàn chỉnh. Những thứ không làm — pack files, delta encoding, remote operations — không phải vì khó, mà vì chúng không liên quan đến 3 concept cốt lõi mà dự án nhắm tới.

Mục tiêu ngay từ đầu không phải là tạo ra một tool thay thế Git. Mục tiêu là hiểu Git đủ sâu để không bao giờ phải đoán nữa.

Mục tiêu đó đã đạt được.
