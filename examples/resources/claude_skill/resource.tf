resource "claude_skill" "pdf_filler" {
  display_title = "PDF Form Filler"

  # All files must share one top-level directory and include a SKILL.md at its
  # root. Read each file with the file() function.
  files = {
    "pdf-filler/SKILL.md"   = file("${path.module}/pdf-filler/SKILL.md")
    "pdf-filler/scripts.py" = file("${path.module}/pdf-filler/scripts.py")
  }
}
