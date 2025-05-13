import pandas as pd
import argparse
import numpy as np
import matplotlib.pyplot as plt
from collections import Counter
from scipy.stats import linregress

parser = argparse.ArgumentParser(
    description='Compute and plot the Zipfian factor of a number distribution from a CSV file'
)
parser.add_argument('fileName', help='Path to the CSV file containing numbers')
parser.add_argument('--output', '-o', help='Path to save the plot (optional)')
args = parser.parse_args()

# File path - change this to your file
file_path = args.fileName  

# Load raw numbers from CSV (assuming single column or flattening all columns)
try:
    df = pd.read_csv(file_path, header=None)
    raw_numbers = df.values.flatten()
    
    # Filter out non-numeric values and NaN values
    numbers = [float(x) for x in raw_numbers if pd.notnull(x)]
    
    print("Total Numbers: ",len(numbers))
    if len(numbers) == 0:
        raise ValueError("No valid numeric data found in the file.")
        
except Exception as e:
    print(f"Error loading or processing file: {e}")
    exit(1)

# Count frequencies using Counter
frequency_counter = Counter(numbers)
# Find the most occurring number
most_common_number, most_common_count = frequency_counter.most_common(1)[0]
print(f"The most occurring number is {most_common_number} with a count of {most_common_count}.")

# Create a dataframe with the counts
count_df = pd.DataFrame({
    'Value': list(frequency_counter.keys()),
    'Count': list(frequency_counter.values())
})

# Sort by Count in descending order and assign Rank
count_df = count_df.sort_values(by='Count', ascending=False).reset_index(drop=True)
count_df['Rank'] = count_df.index + 1

# Compute log10 of Rank and Count
log_rank = np.log10(count_df['Rank'])
log_count = np.log10(count_df['Count'])

# Linear regression to estimate Zipf slope
slope, intercept, r_value, p_value, std_err = linregress(log_rank, log_count)

# Calculate the Zipfian exponent (conventionally positive)
zipf_exponent = -slope

# Plot
plt.figure(figsize=(10, 7))
plt.scatter(log_rank, log_count, alpha=0.7, label='Data')
plt.plot(log_rank, slope * log_rank + intercept, 'r-', linewidth=2, 
         label=f'Fit: slope={slope:.2f} (s={zipf_exponent:.2f})')
plt.title('Zipf Distribution Analysis', fontsize=14)
plt.xlabel('log10(Rank)', fontsize=12)
plt.ylabel('log10(Count)', fontsize=12)
plt.legend(fontsize=11)
plt.grid(True, alpha=0.3)

# Add text box with statistics
stats_text = f"Zipf exponent (s): {zipf_exponent:.4f}\nR-squared: {r_value**2:.4f}"
plt.annotate(stats_text, xy=(0.05, 0.05), xycoords='axes fraction', 
             bbox=dict(boxstyle="round,pad=0.5", fc="white", ec="gray", alpha=0.8))

plt.tight_layout()

# Save plot (optional)
# plt.savefig('zipf_plot.png', dpi=300)

# Show plot
plt.show()

# Print results
print(f"Estimated Zipf exponent (s): {zipf_exponent:.4f}")
print(f"R-squared: {r_value**2:.4f}")

# Option to export the ranked data to CSV
# count_df.to_csv('ranked_data.csv', index=False)