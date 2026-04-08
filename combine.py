# Read niches from queries.txt
with open("queries.txt", "r") as f:
    niches = [line.strip() for line in f if line.strip()]

# Read cities from cities.txt
with open("cities2.txt", "r") as f:
    cities = [line.strip() for line in f if line.strip()]

# Combine niches and cities
combined = []
for niche in niches:
    for city in cities:
        combined.append(f"{niche} in {city.lower()}, scotland")

# Save to output.txt
with open("output.txt", "w") as f:
    for line in combined:
        f.write(line + "\n")

print(f"Generated {len(combined)} combined queries in output.txt")

