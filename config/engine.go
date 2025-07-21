package config

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// Generator interface for all parameter generators
type Generator interface {
	Generate() (any, error)
}

// StaticGenerator generates static values
type StaticGenerator struct {
	Value any
}

func (g *StaticGenerator) Generate() (any, error) {
	return g.Value, nil
}

// RandomIntGenerator generates random integers (always returns int)
type RandomIntGenerator struct {
	Min int
	Max int
}

func (g *RandomIntGenerator) Generate() (any, error) {
	if g.Min >= g.Max {
		return g.Min, nil
	}
	
	// Generate random number in range
	rangeSize := g.Max - g.Min + 1
	num, err := rand.Int(rand.Reader, big.NewInt(int64(rangeSize)))
	if err != nil {
		return nil, fmt.Errorf("failed to generate random int: %w", err)
	}
	
	return g.Min + int(num.Int64()), nil
}

// FormattedIntGenerator generates formatted integers (always returns string)
type FormattedIntGenerator struct {
	Min    int
	Max    int
	Format string
}

func (g *FormattedIntGenerator) Generate() (any, error) {
	if g.Min >= g.Max {
		return fmt.Sprintf(strings.ReplaceAll(g.Format, "{}", "%d"), g.Min), nil
	}
	
	// Generate random number in range
	rangeSize := g.Max - g.Min + 1
	num, err := rand.Int(rand.Reader, big.NewInt(int64(rangeSize)))
	if err != nil {
		return nil, fmt.Errorf("failed to generate random int: %w", err)
	}
	
	value := g.Min + int(num.Int64())
	
	// Apply format
	return fmt.Sprintf(strings.ReplaceAll(g.Format, "{}", "%d"), value), nil
}

// ChoiceGenerator selects from predefined values with optional weights
type ChoiceGenerator struct {
	Values  []any
	Weights []float64
}

func (g *ChoiceGenerator) Generate() (any, error) {
	if len(g.Values) == 0 {
		return nil, fmt.Errorf("no values provided for choice generator")
	}
	
	// Weighted selection if weights are provided
	if len(g.Weights) > 0 && len(g.Weights) == len(g.Values) {
		return g.generateWeightedChoice()
	}
	
	// Simple random choice
	num, err := rand.Int(rand.Reader, big.NewInt(int64(len(g.Values))))
	if err != nil {
		return nil, fmt.Errorf("failed to generate choice: %w", err)
	}
	
	return g.Values[num.Int64()], nil
}

func (g *ChoiceGenerator) generateWeightedChoice() (any, error) {
	// Calculate total weight
	totalWeight := 0.0
	for _, weight := range g.Weights {
		totalWeight += weight
	}
	
	if totalWeight <= 0 {
		return g.Values[0], nil
	}
	
	// Generate random number between 0 and totalWeight
	num, err := rand.Int(rand.Reader, big.NewInt(int64(totalWeight*1000)))
	if err != nil {
		return nil, fmt.Errorf("failed to generate weighted choice: %w", err)
	}
	
	target := float64(num.Int64()) / 1000.0
	cumulative := 0.0
	
	for i, weight := range g.Weights {
		cumulative += weight
		if target <= cumulative {
			return g.Values[i], nil
		}
	}
	
	// Fallback to last value
	return g.Values[len(g.Values)-1], nil
}

// RandomStringGenerator generates random strings
type RandomStringGenerator struct {
	Length  int
	Charset string
}

func (g *RandomStringGenerator) Generate() (any, error) {
	charset := getCharset(g.Charset)
	
	result := make([]byte, g.Length)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return nil, fmt.Errorf("failed to generate random string: %w", err)
		}
		result[i] = charset[num.Int64()]
	}
	
	return string(result), nil
}

// TemplateGenerator generates values from templates
type TemplateGenerator struct {
	Template   string
	Parameters map[string]Generator
}

func (g *TemplateGenerator) Generate() (any, error) {
	if g.Template == "" {
		return nil, fmt.Errorf("no template provided for template generator")
	}
	
	result := g.Template
	
	// Replace template variables
	for paramName, paramGen := range g.Parameters {
		placeholder := fmt.Sprintf("{{%s}}", paramName)
		
		value, err := paramGen.Generate()
		if err != nil {
			return nil, fmt.Errorf("failed to generate template parameter %s: %w", paramName, err)
		}
		
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
	}
	
	return result, nil
}

// ObjectGenerator generates complex objects
type ObjectGenerator struct {
	Properties map[string]Generator
}

func (g *ObjectGenerator) Generate() (any, error) {
	result := make(map[string]any)
	
	for fieldName, fieldGen := range g.Properties {
		value, err := fieldGen.Generate()
		if err != nil {
			return nil, fmt.Errorf("failed to generate object field %s: %w", fieldName, err)
		}
		
		result[fieldName] = value
	}
	
	return result, nil
}

// ReferenceGenerator references a named generator by name and resolves it at generation time
type ReferenceGenerator struct {
	Name   string
	Config *Config
}

func (g *ReferenceGenerator) Generate() (any, error) {
	// Get the referenced generator from config
	if genDef, exists := g.Config.ParameterGenerators[g.Name]; exists {
		gen, err := g.Config.createGeneratorFromDef(genDef)
		if err != nil {
			return nil, fmt.Errorf("failed to create referenced generator '%s': %w", g.Name, err)
		}
		return gen.Generate()
	}
	return nil, fmt.Errorf("referenced generator '%s' not found", g.Name)
}

// ArrayGenerator generates arrays of values
type ArrayGenerator struct {
	MinLength        int
	MaxLength        int
	ElementGenerator Generator
}

func (g *ArrayGenerator) Generate() (any, error) {
	if g.MinLength > g.MaxLength {
		g.MinLength = g.MaxLength
	}
	
	// Generate random length
	lengthRange := g.MaxLength - g.MinLength + 1
	num, err := rand.Int(rand.Reader, big.NewInt(int64(lengthRange)))
	if err != nil {
		return nil, fmt.Errorf("failed to generate array length: %w", err)
	}
	length := g.MinLength + int(num.Int64())
	
	var result []any
	
	for i := 0; i < length; i++ {
		value, err := g.ElementGenerator.Generate()
		if err != nil {
			return nil, fmt.Errorf("failed to generate array element %d: %w", i, err)
		}
		result = append(result, value)
	}
	
	return result, nil
}

// ParameterEngine manages generator creation and execution
type ParameterEngine struct {
	generators map[string]Generator
}

// NewParameterEngine creates a new parameter generation engine
func NewParameterEngine() *ParameterEngine {
	return &ParameterEngine{
		generators: make(map[string]Generator),
	}
}

// GenerateValue generates a value using a named generator or creates one from definition
func (pe *ParameterEngine) GenerateValue(genDef any, name string) (any, error) {
	// Check if it's a reference to a named generator
	if genName, ok := genDef.(string); ok {
		if gen, exists := pe.generators[genName]; exists {
			return gen.Generate()
		}
		return nil, fmt.Errorf("generator '%s' not found", genName)
	}
	
	// Create generator from definition
	gen, err := pe.createGenerator(genDef)
	if err != nil {
		return nil, fmt.Errorf("failed to create generator: %w", err)
	}
	
	return gen.Generate()
}

// RegisterGenerator registers a named generator
func (pe *ParameterEngine) RegisterGenerator(name string, gen Generator) {
	pe.generators[name] = gen
}

// GetGenerator retrieves a named generator
func (pe *ParameterEngine) GetGenerator(name string) (Generator, bool) {
	gen, exists := pe.generators[name]
	return gen, exists
}

// createGenerator creates a generator from a definition map
func (pe *ParameterEngine) createGenerator(def any) (Generator, error) {
	return pe.createGeneratorWithConfig(def, nil)
}

// createGeneratorWithConfig creates a generator from a definition map with access to config for references
func (pe *ParameterEngine) createGeneratorWithConfig(def any, config *Config) (Generator, error) {
	// Handle string values as static generators
	if str, ok := def.(string); ok {
		return &StaticGenerator{Value: str}, nil
	}
	
	// Convert to map for parsing
	var defMap map[string]any
	
	switch v := def.(type) {
	case map[interface{}]interface{}:
		defMap = make(map[string]any)
		for k, val := range v {
			if keyStr, ok := k.(string); ok {
				defMap[keyStr] = val
			}
		}
	case map[string]any:
		defMap = v
	default:
		return nil, fmt.Errorf("invalid generator definition type: %T", def)
	}
	
	// Check for $ref reference syntax
	if refName, exists := defMap["$ref"]; exists {
		if genName, ok := refName.(string); ok {
			if config != nil {
				// Create a reference generator that resolves at generation time
				return &ReferenceGenerator{
					Name:   genName,
					Config: config,
				}, nil
			}
			// Fallback to engine's registered generators
			if gen, exists := pe.generators[genName]; exists {
				return gen, nil
			}
			return nil, fmt.Errorf("referenced generator '%s' not found", genName)
		}
		return nil, fmt.Errorf("$ref value must be a string")
	}
	
	genType, ok := defMap["type"].(string)
	if !ok {
		return nil, fmt.Errorf("generator type not specified")
	}
	
	switch genType {
	case "static":
		return &StaticGenerator{
			Value: defMap["value"],
		}, nil
		
	case "randomInt":
		min := getIntValue(defMap["min"], 0)
		max := getIntValue(defMap["max"], 100)
		format := getStringValue(defMap["format"], "")
		
		if format != "" {
			return &FormattedIntGenerator{
				Min:    min,
				Max:    max,
				Format: format,
			}, nil
		} else {
			return &RandomIntGenerator{
				Min: min,
				Max: max,
			}, nil
		}
		
	case "formattedInt":
		min := getIntValue(defMap["min"], 0)
		max := getIntValue(defMap["max"], 100)
		format := getStringValue(defMap["format"], "{}")
		
		if format == "" {
			return nil, fmt.Errorf("formattedInt generator requires 'format' field")
		}
		
		return &FormattedIntGenerator{
			Min:    min,
			Max:    max,
			Format: format,
		}, nil
		
	case "choice":
		values, ok := defMap["values"].([]any)
		if !ok {
			// Try to convert from []interface{} if needed
			if interfaceValues, ok := defMap["values"].([]interface{}); ok {
				values = interfaceValues
			} else {
				return nil, fmt.Errorf("choice generator requires 'values' field")
			}
		}
		
		var weights []float64
		if w, exists := defMap["weights"]; exists {
			if weightList, ok := w.([]interface{}); ok {
				for _, weight := range weightList {
					weights = append(weights, getFloatValue(weight, 0.0))
				}
			}
		}
		
		return &ChoiceGenerator{
			Values:  values,
			Weights: weights,
		}, nil
		
	case "randomString":
		length := getIntValue(defMap["length"], 10)
		charset := getStringValue(defMap["charset"], "alphanumeric")
		
		return &RandomStringGenerator{
			Length:  length,
			Charset: charset,
		}, nil
		
	case "template":
		template := getStringValue(defMap["template"], "")
		if template == "" {
			return nil, fmt.Errorf("template generator requires 'template' field")
		}
		
		parameters := make(map[string]Generator)
		if params, exists := defMap["parameters"]; exists {
			if paramMap, ok := params.(map[interface{}]interface{}); ok {
				for k, v := range paramMap {
					if keyStr, ok := k.(string); ok {
						subGen, err := pe.createGeneratorWithConfig(v, config)
						if err != nil {
							return nil, fmt.Errorf("failed to create template parameter %s: %w", keyStr, err)
						}
						parameters[keyStr] = subGen
					}
				}
			}
		}
		
		return &TemplateGenerator{
			Template:   template,
			Parameters: parameters,
		}, nil
		
	case "object":
		properties := make(map[string]Generator)
		if props, exists := defMap["properties"]; exists {
			if propMap, ok := props.(map[interface{}]interface{}); ok {
				for k, v := range propMap {
					if keyStr, ok := k.(string); ok {
						subGen, err := pe.createGeneratorWithConfig(v, config)
						if err != nil {
							return nil, fmt.Errorf("failed to create object property %s: %w", keyStr, err)
						}
						properties[keyStr] = subGen
					}
				}
			}
		}
		
		return &ObjectGenerator{
			Properties: properties,
		}, nil
		
	case "array":
		minLength := getIntValue(defMap["minLength"], 1)
		maxLength := getIntValue(defMap["maxLength"], 5)
		
		elemGenDef, exists := defMap["elementGenerator"]
		if !exists {
			return nil, fmt.Errorf("array generator requires 'elementGenerator' field")
		}
		
		elemGen, err := pe.createGeneratorWithConfig(elemGenDef, config)
		if err != nil {
			return nil, fmt.Errorf("failed to create array element generator: %w", err)
		}
		
		return &ArrayGenerator{
			MinLength:        minLength,
			MaxLength:        maxLength,
			ElementGenerator: elemGen,
		}, nil
		
	default:
		return nil, fmt.Errorf("unsupported generator type: %s", genType)
	}
}

// Helper functions

func getCharset(charsetType string) string {
	switch charsetType {
	case "alpha":
		return "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	case "alpha_lower":
		return "abcdefghijklmnopqrstuvwxyz"
	case "alpha_upper":
		return "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	case "numeric":
		return "0123456789"
	case "alphanumeric":
		return "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	case "hex":
		return "0123456789abcdef"
	default:
		return "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	}
}

func getIntValue(value any, defaultValue int) int {
	switch v := value.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case string:
		if intVal, err := strconv.Atoi(v); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getFloatValue(value any, defaultValue float64) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		if floatVal, err := strconv.ParseFloat(v, 64); err == nil {
			return floatVal
		}
	}
	return defaultValue
}

func getStringValue(value any, defaultValue string) string {
	if str, ok := value.(string); ok {
		return str
	}
	return defaultValue
}
