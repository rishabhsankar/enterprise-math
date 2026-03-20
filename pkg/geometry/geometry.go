package geometry

import (
	"fmt"
	"math"
	"sort"
	"sync"
)

// Point represents a 2D coordinate.
type Point struct {
	X, Y float64
}

// Distance returns the Euclidean distance between two points.
func Distance(a, b Point) float64 {
	dx := b.X - a.X
	dy := b.Y - a.Y
	return math.Sqrt(dx*dx + dy*dy)
}

// ManhattanDistance returns the Manhattan distance between two points.
func ManhattanDistance(a, b Point) float64 {
	return math.Abs(b.X-a.X) + math.Abs(b.Y-a.Y)
}

// Polygon represents a closed polygon defined by its vertices.
type Polygon struct {
	vertices []Point
	mu       sync.RWMutex
	area     float64
	areaDone bool
}

// NewPolygon creates a new polygon from the given vertices.
func NewPolygon(vertices []Point) *Polygon {
	copied := make([]Point, len(vertices))
	copy(copied, vertices)
	return &Polygon{vertices: copied}
}

// Vertices returns a copy of the polygon vertices.
func (p *Polygon) Vertices() []Point {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]Point, len(p.vertices))
	copy(out, p.vertices)
	return out
}

// NumVertices returns the number of vertices.
func (p *Polygon) NumVertices() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.vertices)
}

// Area computes the area of a simple polygon using the shoelace formula.
func (p *Polygon) Area() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.areaDone {
		return p.area
	}
	n := len(p.vertices)
	if n < 3 {
		return 0
	}
	sum := 0.0
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		sum += p.vertices[i].X * p.vertices[j].Y
		sum -= p.vertices[j].X * p.vertices[i].Y
	}
	p.area = math.Abs(sum) / 2.0
	p.areaDone = true
	return p.area
}

// Perimeter computes the perimeter of the polygon.
func (p *Polygon) Perimeter() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	n := len(p.vertices)
	if n < 2 {
		return 0
	}
	total := 0.0
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		total += Distance(p.vertices[i], p.vertices[j])
	}
	return total
}

// Centroid returns the geometric center of the polygon.
func (p *Polygon) Centroid() Point {
	p.mu.RLock()
	defer p.mu.RUnlock()
	n := len(p.vertices)
	if n == 0 {
		return Point{}
	}
	var cx, cy float64
	for _, v := range p.vertices {
		cx += v.X
		cy += v.Y
	}
	return Point{X: cx / float64(n), Y: cy / float64(n)}
}

// ContainsPoint checks if a point is inside the polygon using ray casting.
func (p *Polygon) ContainsPoint(pt Point) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	n := len(p.vertices)
	if n < 3 {
		return false
	}
	inside := false
	j := n - 1
	for i := 0; i < n; i++ {
		vi := p.vertices[i]
		vj := p.vertices[j]
		if (vi.Y > pt.Y) != (vj.Y > pt.Y) &&
			pt.X < (vj.X-vi.X)*(pt.Y-vi.Y)/(vj.Y-vi.Y)+vi.X {
			inside = !inside
		}
		j = i
	}
	return inside
}

// IsConvex returns true if the polygon is convex.
func (p *Polygon) IsConvex() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	n := len(p.vertices)
	if n < 3 {
		return false
	}
	var sign bool
	signSet := false
	for i := 0; i < n; i++ {
		a := p.vertices[i]
		b := p.vertices[(i+1)%n]
		c := p.vertices[(i+2)%n]
		cross := (b.X-a.X)*(c.Y-b.Y) - (b.Y-a.Y)*(c.X-b.X)
		if cross != 0 {
			if !signSet {
				sign = cross > 0
				signSet = true
			} else if (cross > 0) != sign {
				return false
			}
		}
	}
	return true
}

// AddVertex appends a vertex to the polygon and invalidates cached area.
func (p *Polygon) AddVertex(v Point) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.vertices = append(p.vertices, v)
	p.areaDone = false
}

// BoundingBox returns the min and max corners of the axis-aligned bounding box.
func (p *Polygon) BoundingBox() (Point, Point) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if len(p.vertices) == 0 {
		return Point{}, Point{}
	}
	minP := p.vertices[0]
	maxP := p.vertices[0]
	for _, v := range p.vertices[1:] {
		if v.X < minP.X {
			minP.X = v.X
		}
		if v.Y < minP.Y {
			minP.Y = v.Y
		}
		if v.X > maxP.X {
			maxP.X = v.X
		}
		if v.Y > maxP.Y {
			maxP.Y = v.Y
		}
	}
	return minP, maxP
}

// ConvexHull returns the convex hull of a set of points using Graham scan.
func ConvexHull(points []Point) []Point {
	if len(points) < 3 {
		return points
	}

	// Find the lowest point (and leftmost if tied)
	pivot := 0
	for i := 1; i < len(points); i++ {
		if points[i].Y < points[pivot].Y ||
			(points[i].Y == points[pivot].Y && points[i].X < points[pivot].X) {
			pivot = i
		}
	}
	points[0], points[pivot] = points[pivot], points[0]
	p0 := points[0]

	// Sort by polar angle
	sort.Slice(points[1:], func(i, j int) bool {
		a := points[i+1]
		b := points[j+1]
		cross := (a.X-p0.X)*(b.Y-p0.Y) - (a.Y-p0.Y)*(b.X-p0.X)
		if cross == 0 {
			da := (a.X-p0.X)*(a.X-p0.X) + (a.Y-p0.Y)*(a.Y-p0.Y)
			db := (b.X-p0.X)*(b.X-p0.X) + (b.Y-p0.Y)*(b.Y-p0.Y)
			return da < db
		}
		return cross > 0
	})

	stack := []Point{points[0], points[1], points[2]}
	for i := 3; i < len(points); i++ {
		for len(stack) > 1 {
			top := stack[len(stack)-1]
			second := stack[len(stack)-2]
			cross := (top.X-second.X)*(points[i].Y-second.Y) - (top.Y-second.Y)*(points[i].X-second.X)
			if cross <= 0 {
				stack = stack[:len(stack)-1]
			} else {
				break
			}
		}
		stack = append(stack, points[i])
	}
	return stack
}

// Circle represents a circle with center and radius.
type Circle struct {
	Center Point
	Radius float64
}

// CircleArea returns the area of a circle.
func (c Circle) Area() float64 {
	return math.Pi * c.Radius * c.Radius
}

// CircleCircumference returns the circumference of a circle.
func (c Circle) Circumference() float64 {
	return 2 * math.Pi * c.Radius
}

// ContainsPoint checks if a point is inside the circle.
func (c Circle) ContainsPoint(pt Point) bool {
	return Distance(c.Center, pt) <= c.Radius
}

// Intersects checks if two circles intersect.
func (c Circle) Intersects(other Circle) bool {
	d := Distance(c.Center, other.Center)
	return d <= c.Radius+other.Radius && d >= math.Abs(c.Radius-other.Radius)
}

// String returns a string representation of a Point.
func (p Point) String() string {
	return fmt.Sprintf("(%.2f, %.2f)", p.X, p.Y)
}

// String returns a string representation of a Circle.
func (c Circle) String() string {
	return fmt.Sprintf("Circle{center=%s, r=%.2f}", c.Center, c.Radius)
}
