resource "claude_skill" "pdf_filler" {
  display_title = "PDF Form Filler"
}

# Each claude_skill_version uploads a new immutable version of the skill.
resource "claude_skill_version" "pdf_filler_current" {
  skill_id = claude_skill.pdf_filler.id

  files = {
    "pdf-filler/SKILL.md"           = file("${path.module}/pdf-filler/SKILL.md")
    "pdf-filler/scripts/example.py" = file("${path.module}/pdf-filler/scripts/example.py")
  }
}
