.DEFAULT_GOAL = all

all: ppt doc

doc:
	@mkdir -p output/chaps
	@xelatex -output-directory output doc.tex
	@xelatex -output-directory output doc.tex

ppt:
	@mkdir -p output
	@xelatex -output-directory output ppt.tex
	@xelatex -output-directory output ppt.tex

.PHONY: clean

clean:
	@rm -rf output/*
	@rm -rf output/chaps
