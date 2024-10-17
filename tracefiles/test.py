import re

# Path to the input and output file
input_file_path = './serverInput.txt'
output_file_path = './serverInputNew.txt'

# Regular expressions to match the title and description lines
title_pattern = re.compile(r'(SET item/title/\d+ )(.+)')
description_pattern = re.compile(r'(SET item/description/\d+ )(.+)')
comment_pattern = re.compile(r'(SET review/comment/\d+ )(.+)')

# Open the input file for reading and the output file for writing
with open(input_file_path, 'r') as infile, open(output_file_path, 'w') as outfile:
    for line in infile:
        # Replace the title and description with "test" if the line contains them
        line = re.sub(title_pattern, r'\1testTitle', line)
        line = re.sub(description_pattern, r'\1testDesc', line)
        line = re.sub(comment_pattern, r'\1testComment', line)
        outfile.write(line)

print(f"All titles and descriptions have been replaced with 'test'. The updated file is saved as {output_file_path}.")