// External JavaScript file for data validation and processing
// This script demonstrates:
// 1. Accessing state from previous HTTP and script steps
// 2. Performing complex data validation and transformation
// 3. Saving processed data for subsequent HTTP steps to use

console.log("=== EXTERNAL JS FILE: validate-and-process.js ===");

// Validate that we have all expected data from previous steps
console.log("Validating data from previous steps...");

// Check HTTP step data
if (!state.animal_name || !state.animal_species) {
    assert(false, "Missing animal data from HTTP step");
}

// Check script step 1 data  
if (!state.processed_user_name || !state.step1_completed) {
    assert(false, "Missing data from script step 1");
}

console.log("✓ All required data present from previous steps");

// Access and validate config variables
console.log("Config variables:");
console.log("- API URL:", vars.api_url);
console.log("- User name:", vars.user_name);
console.log("- Max retries:", vars.max_retries);

// Process animal data with business logic
const animalName = state.animal_name;
const animalSpecies = state.animal_species;
const userName = state.processed_user_name;

// Complex business logic processing
let animalCategory = "unknown";
const domesticAnimals = ["dog", "cat", "horse", "cow", "pig", "chicken"];
const wildAnimals = ["lion", "tiger", "elephant", "bear", "wolf", "eagle"];

if (domesticAnimals.some(animal => animalName.toLowerCase().includes(animal))) {
    animalCategory = "domestic";
} else if (wildAnimals.some(animal => animalName.toLowerCase().includes(animal))) {
    animalCategory = "wild";
} else {
    animalCategory = "exotic";
}

// Calculate animal score based on species and category
let animalScore = animalName.length + animalSpecies.length;
if (animalCategory === "wild") animalScore += 10;
if (animalCategory === "exotic") animalScore += 15;

// Generate recommendations based on processed data
let recommendations = [];
if (animalCategory === "domestic") {
    recommendations.push("suitable_for_families");
    recommendations.push("requires_regular_care");
} else if (animalCategory === "wild") {
    recommendations.push("observe_from_distance");
    recommendations.push("conservation_important");
} else {
    recommendations.push("special_care_needed");
    recommendations.push("research_requirements");
}

// Create a comprehensive animal profile
const animalProfile = {
    name: animalName,
    species: animalSpecies,
    category: animalCategory,
    score: animalScore,
    assessedBy: userName,
    assessmentDate: new Date().toISOString(),
    recommendations: recommendations
};

// Save processed data for subsequent HTTP steps to use
save("animal_category", animalCategory);
save("animal_score", animalScore.toString());
save("animal_profile", JSON.stringify(animalProfile));
save("recommendations_count", recommendations.length.toString());
save("assessment_complete", "true");
save("processing_timestamp", new Date().toISOString());

// Save individual recommendations for easy HTTP template access
recommendations.forEach((rec, index) => {
    save(`recommendation_${index + 1}`, rec);
});

// Generate data that HTTP steps can use in URL templates
save("search_category", animalCategory);
save("score_range", animalScore > 20 ? "high" : animalScore > 10 ? "medium" : "low");

// Validation assertions
assert(animalScore > 0, "Animal score should be positive");
assert(animalCategory !== "unknown", "Animal category should be determined");
assert(recommendations.length > 0, "Should have at least one recommendation");

console.log("✓ External JS processing completed successfully");
console.log("- Animal category:", animalCategory);
console.log("- Animal score:", animalScore);
console.log("- Recommendations:", recommendations.length);
console.log("=== END EXTERNAL JS FILE ===");