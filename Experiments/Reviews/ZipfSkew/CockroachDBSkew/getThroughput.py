import os
import json

def calculate_average_throughput(directory):
    throughput_values = []
    
    # Iterate through all files in the directory
    for filename in os.listdir(directory):
        if filename.endswith(".json"):
            file_path = os.path.join(directory, filename)
            
            # Open and read the JSON file
            with open(file_path, 'r') as file:
                try:
                    data = json.load(file)
                    # Extract the throughput value
                    throughput = data.get("Throughput (requests/second)", None)
                    if throughput is not None:
                        throughput_values.append(throughput)
                except json.JSONDecodeError:
                    print(f"Error decoding JSON in file: {filename}")
    
    # Calculate the average throughput
    if throughput_values:
        average_throughput = sum(throughput_values) / len(throughput_values)
        return average_throughput
    else:
        return None

if __name__ == "__main__":
    # Replace these with your actual directories
    directories = [
        "./skewed",
        "./uniform_results"
    ]
    
    for directory in directories:
        average_throughput = calculate_average_throughput(directory)
        if average_throughput is not None:
            print(f"Average Throughput for '{directory}': {average_throughput:.2f} requests/second")
        else:
            print(f"No throughput data found in directory: '{directory}'")
