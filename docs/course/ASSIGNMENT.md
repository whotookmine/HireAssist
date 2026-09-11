# Assignment — Project Proposal & ADRs

The brief and submission guideline as given by the lecturer. Recorded here because it is the
specification `docs/PROPOSAL.md` is written against, and it otherwise exists only in email and
in class.

**Worth:** Project proposal 5% + ADR 3% of the course total (see [GRADING.md](GRADING.md)).

---

## The brief

> Submit your project description and ADRs, including these information:
>
> - Project Name:
> - Group Members:
> - Problem Description:
> - Target Customers:
> - Scenario (use-case & description)
> - Functional Requirements
> - Non-functional Requirements
> - ADRs (at least 3, sample in Supplementary Materials)

`docs/PROPOSAL.md` uses these exact headings, in this order. Do not rename or reorder them.

The Functional Requirements, Non-functional Requirements and ADRs sections hold a summary or an
index and point to `FUNCTIONAL-REQUIREMENTS.md`, `NON-FUNCTIONAL-REQUIREMENTS.md` and `adr/`,
which carry the full content. **Before submitting, inline all three back into the proposal** —
the brief asks for one document, and a marker should not have to follow links.

The "sample in Supplementary Materials" is in [`../reference/`](../reference/) —
`adr-sample-good.pdf` and `adr-sample-bad.pdf`.

---

## Submission guideline

### 1. Project Proposal

สามารถ แก้ไข ปรับปรุง หรือเพิ่มเติม Proposal จากที่ทำใน Startup Workshop ได้ เพื่อให้รายละเอียดของโครงการชัดเจนและ
เหมาะสมยิ่งขึ้น แต่เนื้อหาหลักของโครงการควรยังอยู่ภายใน scope ของ project ที่ได้กำหนดไว้

อย่างไรก็ตาม การแก้ไข Proposal ในครั้งนี้ จะไม่มีผลย้อนกลับไปยังคะแนนของ Startup Workshop เนื่องจากส่วนนั้นได้ตรวจและ
ให้คะแนนเรียบร้อยแล้ว

> *The proposal from the Startup Workshop may be revised, improved or extended so the project
> is clearer and better specified — but the core content must stay within the scope already
> defined. Revising it now does not retroactively change the Startup Workshop score, which has
> already been marked.*

**What this means for us:** we are free to sharpen the proposal, and we have — the customer
definition, the use case set, and the deferral of talent-pool re-matching all changed. What we
must not do is switch to a different project.

### 2. Use Cases

ให้ระบุ Use Case หลักอย่างน้อย 3 Use Cases ที่สะท้อนการทำงานสำคัญของระบบ

สำหรับแต่ละ Use Case ขอให้เขียนรายละเอียดให้ผู้อ่านสามารถเข้าใจได้ว่า

- ใครเป็นผู้ใช้งานหรือ Actor
- ต้องการทำอะไร
- ระบบมีการทำงานหรือโต้ตอบอย่างไรในภาพรวม
- ผลลัพธ์ที่ผู้ใช้ต้องการได้รับคืออะไร

ไม่จำเป็นต้องเขียนละเอียดถึงระดับ specification เต็มรูปแบบ แต่ต้องมีรายละเอียดเพียงพอที่จะทำให้เข้าใจ พฤติกรรมและขอบเขต
ของระบบ ได้อย่างชัดเจน หากเขียนสั้นหรือกำกวมจนไม่สามารถเข้าใจได้ว่า Use Case นั้นทำอะไร **อาจถูกหักคะแนนได้**

หากไม่แน่ใจว่า "ควรใส่รายละเอียดมากน้อยเพียงใด" แนะนำให้ ใส่รายละเอียดไว้ก่อนเท่าที่จำเป็นต่อความเข้าใจ ดีกว่าเขียนสั้นจน
ตีความไม่ได้

> *At least 3 main use cases reflecting the system's important behaviour. Each must let the
> reader understand: **who** the actor is, **what** they want to do, **how** the system
> responds overall, and **what result** the user gets. Full formal specification is not
> required, but there must be enough detail to make the system's behaviour and scope clear.
> Writing so briefly or ambiguously that the use case cannot be understood **may lose marks.**
> When unsure how much detail to include, err on the side of more.*

**What this means for us:** every use case in `docs/PROPOSAL.md` carries Actor / Goal /
Precondition / a prose Description / Main flow / Alternate flows / Outcome — the four required
questions plus the failure paths. Terseness is the specific risk called out here, so do not
compress these sections to save space.

### 3. Use Case Diagram

ให้จัดทำ Use Case Diagram ให้สอดคล้องกับ Use Cases ที่อธิบายไว้

สำหรับความสัมพันธ์ เช่น `<<include>>` หรือ `<<extend>>` ไม่ได้กำหนดว่าทุกโครงการจะต้องมี ให้นิสิตพิจารณาตามลักษณะของ
ระบบและใช้เมื่อมีความหมายตามหลักของ Use Case Modeling จริง ๆ

ไม่ควรใส่ include หรือ extend เพียงเพื่อทำให้ diagram ดูซับซ้อนขึ้น แต่ในทางกลับกัน หากระบบมีความสัมพันธ์ลักษณะดังกล่าว
อย่างชัดเจน ก็ควรแสดงให้เหมาะสม

หลักสำคัญคือ: จาก Project Description, Use Cases และ Use Case Diagram ผู้ตรวจควรสามารถเข้าใจได้ว่า **ระบบนี้ทำอะไร
ใครใช้งาน และขอบเขตของระบบอยู่ตรงไหน**

> *Produce a use case diagram consistent with the use cases described. `<<include>>` and
> `<<extend>>` are **not required** — use them only where they genuinely apply under use case
> modelling principles. Do not add them merely to make the diagram look more complex; but if
> the system clearly has such relationships, show them. The key principle: from the project
> description, use cases and diagram, the marker should understand **what the system does, who
> uses it, and where its boundary lies.***

**What this means for us:** this is why the diagram carries no `«include»` or `«extend»`
relationships, and why the rationale below it states positively why there are none. A diagram
element must also be backed by the use cases, which is why the `«extend» Draft Interview
Invitation Email` was removed once it turned out no use case or requirement described it.

---

## Status against the brief

| Required | Where | Status |
|---|---|---|
| Project Name | `PROPOSAL.md` | Done |
| Group Members | `PROPOSAL.md` | Done |
| Problem Description | `PROPOSAL.md` | Done |
| Target Customers | `PROPOSAL.md` | Done |
| Scenario (use-case & description) | `PROPOSAL.md` | Done — 7 use cases, above the minimum of 3 |
| Use Case Diagram | `PROPOSAL.md` → `diagrams/use-case-diagram.puml` | Done — UML notation, PlantUML source with rendered SVG |
| Functional Requirements | `PROPOSAL.md` → `FUNCTIONAL-REQUIREMENTS.md` | Done — 44, traced to use cases |
| Non-functional Requirements | `PROPOSAL.md` → `NON-FUNCTIONAL-REQUIREMENTS.md` | Done — 17, grouped by quality attribute |
| ADRs (≥3) | `../adr/` | Done — 6 recorded (ADR-001…006; ADR-005 *Proposed*), indexed from `PROPOSAL.md` |
