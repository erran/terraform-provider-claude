# Skills can be imported by their skill_ identifier. Uploaded files are
# write-only and are not restored on import, so the next plan will show a diff
# for the files attribute; applying it uploads a new version (it does not
# replace the skill).
terraform import claude_skill.pdf_filler skill_0123456789abcdef
